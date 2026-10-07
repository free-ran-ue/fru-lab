// Package xdp gives the data plane an AF_XDP path: a small XDP program
// that hands fru-tester's own UDP packets to AF_XDP sockets (everything
// else goes on to the kernel), the sockets themselves, and next-hop MAC
// lookup for building whole Ethernet frames.
//
// AF_XDP is part of Linux: a NIC stays usable by the kernel, no huge pages
// are needed, and a driver that supports it moves packets without copies
// (zero-copy); other drivers fall back to copy mode, and any interface,
// veths included, can run the program in generic mode. The NIC only
// decides how fast it goes.
package xdp

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/unix"
)

// FrameSize is one UMEM frame, enough for a 1500-MTU frame and headroom.
// Half a socket's frames receive, half transmit.
const (
	FrameSize = 2048
	numFrames = 4096
	ringSize  = 2048
	rxFrames  = numFrames / 2
)

// mmap offsets of the four rings (include/uapi/linux/if_xdp.h).
const (
	pgoffRxRing         = 0
	pgoffTxRing         = 0x80000000
	pgoffFillRing       = 0x100000000
	pgoffCompletionRing = 0x180000000
)

// ring is one of a socket's four rings, shared with the kernel. A
// producer writes entries then publishes its index; a consumer reads up to
// the producer's index then publishes its own.
type ring struct {
	mem      []byte
	producer *uint32
	consumer *uint32
	flags    *uint32
	descs    unsafe.Pointer
	mask     uint32
}

func mapRing(fd int, off unix.XDPRingOffset, pgoff int64, entrySize uintptr) (ring, error) {
	length := int(off.Desc) + ringSize*int(entrySize)
	mem, err := unix.Mmap(fd, pgoff, length, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_POPULATE)
	if err != nil {
		return ring{}, err
	}
	base := unsafe.Pointer(&mem[0])
	return ring{
		mem:      mem,
		producer: (*uint32)(unsafe.Add(base, off.Producer)),
		consumer: (*uint32)(unsafe.Add(base, off.Consumer)),
		flags:    (*uint32)(unsafe.Add(base, off.Flags)),
		descs:    unsafe.Add(base, off.Desc),
		mask:     ringSize - 1,
	}, nil
}

func (r *ring) desc(i uint32) *unix.XDPDesc {
	return (*unix.XDPDesc)(unsafe.Add(r.descs, uintptr(i&r.mask)*unsafe.Sizeof(unix.XDPDesc{})))
}

func (r *ring) addr(i uint32) *uint64 {
	return (*uint64)(unsafe.Add(r.descs, uintptr(i&r.mask)*8))
}

func (r *ring) needWakeup() bool { return atomic.LoadUint32(r.flags)&unix.XDP_RING_NEED_WAKEUP != 0 }

// Socket is one AF_XDP socket bound to one queue of one interface, with
// its own UMEM. One goroutine receives (Receive, Wait); any number may
// send (Queue, Flush), which take turns on the transmit side.
type Socket struct {
	fd       int
	umem     []byte
	rx, fill ring
	tx, comp ring
	// ZeroCopy is set when the driver moves frames without copies.
	ZeroCopy bool

	txMu    sync.Mutex
	txFree  []uint64 // free transmit frames (UMEM offsets)
	txQueue uint32   // entries written since the last Flush
}

// Open binds a socket to queue of ifindex: zero-copy if tryZeroCopy and
// the driver supports it, else copy mode.
func Open(ifindex, queue int, tryZeroCopy bool) (*Socket, error) {
	modes := []uint16{unix.XDP_COPY}
	if tryZeroCopy {
		modes = []uint16{unix.XDP_ZEROCOPY, unix.XDP_COPY}
	}
	var errs []error
	for _, mode := range modes {
		s, err := open(ifindex, queue, mode)
		if err == nil {
			return s, nil
		}
		errs = append(errs, err)
	}
	return nil, errors.Join(errs...)
}

func open(ifindex, queue int, mode uint16) (s *Socket, err error) {
	fd, err := unix.Socket(unix.AF_XDP, unix.SOCK_RAW|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("AF_XDP socket: %w", err)
	}
	s = &Socket{fd: fd, ZeroCopy: mode == unix.XDP_ZEROCOPY}
	defer func() {
		if err != nil {
			s.Close()
		}
	}()
	s.umem, err = unix.Mmap(-1, 0, numFrames*FrameSize, unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_PRIVATE|unix.MAP_ANONYMOUS|unix.MAP_POPULATE)
	if err != nil {
		return s, fmt.Errorf("UMEM: %w", err)
	}
	reg := unix.XDPUmemReg{Addr: uint64(uintptr(unsafe.Pointer(&s.umem[0]))), Len: uint64(len(s.umem)), Size: FrameSize}
	if err := setsockopt(fd, unix.XDP_UMEM_REG, unsafe.Pointer(&reg), unsafe.Sizeof(reg)); err != nil {
		return s, fmt.Errorf("register UMEM: %w", err)
	}
	for _, opt := range []int{unix.XDP_UMEM_FILL_RING, unix.XDP_UMEM_COMPLETION_RING, unix.XDP_RX_RING, unix.XDP_TX_RING} {
		if err := unix.SetsockoptInt(fd, unix.SOL_XDP, opt, ringSize); err != nil {
			return s, fmt.Errorf("ring size: %w", err)
		}
	}
	var off unix.XDPMmapOffsets
	size := uint32(unsafe.Sizeof(off))
	if _, _, errno := unix.Syscall6(unix.SYS_GETSOCKOPT, uintptr(fd), unix.SOL_XDP, unix.XDP_MMAP_OFFSETS,
		uintptr(unsafe.Pointer(&off)), uintptr(unsafe.Pointer(&size)), 0); errno != 0 {
		return s, fmt.Errorf("ring offsets: %w", errno)
	}
	descSize := unsafe.Sizeof(unix.XDPDesc{})
	if s.rx, err = mapRing(fd, off.Rx, pgoffRxRing, descSize); err != nil {
		return s, err
	}
	if s.tx, err = mapRing(fd, off.Tx, pgoffTxRing, descSize); err != nil {
		return s, err
	}
	if s.fill, err = mapRing(fd, off.Fr, pgoffFillRing, 8); err != nil {
		return s, err
	}
	if s.comp, err = mapRing(fd, off.Cr, pgoffCompletionRing, 8); err != nil {
		return s, err
	}
	// hand the kernel every receive frame, keep the rest for sending
	for i := range uint32(rxFrames) {
		*s.fill.addr(i) = uint64(i) * FrameSize
	}
	atomic.StoreUint32(s.fill.producer, rxFrames)
	for i := rxFrames; i < numFrames; i++ {
		s.txFree = append(s.txFree, uint64(i)*FrameSize)
	}
	sa := &unix.SockaddrXDP{Flags: mode | unix.XDP_USE_NEED_WAKEUP, Ifindex: uint32(ifindex), QueueID: uint32(queue)}
	if err := unix.Bind(fd, sa); err != nil {
		return s, fmt.Errorf("bind queue %d (%s): %w", queue, modeName(mode), err)
	}
	return s, nil
}

func modeName(mode uint16) string {
	if mode == unix.XDP_ZEROCOPY {
		return "zero-copy"
	}
	return "copy mode"
}

func setsockopt(fd, opt int, val unsafe.Pointer, size uintptr) error {
	if _, _, errno := unix.Syscall6(unix.SYS_SETSOCKOPT, uintptr(fd), unix.SOL_XDP, uintptr(opt), uintptr(val), size, 0); errno != 0 {
		return errno
	}
	return nil
}

// FD is the socket's descriptor, for the XDP program's socket map.
func (s *Socket) FD() int { return s.fd }

// Receive hands every frame waiting on the rx ring to handle, then gives
// the frames back to the kernel, and reports how many there were. The
// slice is only valid during the call.
func (s *Socket) Receive(handle func(frame []byte)) int {
	prod := atomic.LoadUint32(s.rx.producer)
	cons := atomic.LoadUint32(s.rx.consumer)
	n := prod - cons
	if n == 0 {
		return 0
	}
	fill := atomic.LoadUint32(s.fill.producer)
	for i := range n {
		d := s.rx.desc(cons + i)
		handle(s.umem[d.Addr : d.Addr+uint64(d.Len)])
		// the frame goes straight back: there are as many fill slots as frames
		*s.fill.addr(fill + i) = d.Addr &^ (FrameSize - 1)
	}
	atomic.StoreUint32(s.rx.consumer, prod)
	atomic.StoreUint32(s.fill.producer, fill+n)
	return int(n)
}

// Wait blocks until frames arrive or timeoutMs passes, waking the kernel
// up if it asked for it.
func (s *Socket) Wait(timeoutMs int) {
	fds := []unix.PollFd{{Fd: int32(s.fd), Events: unix.POLLIN}}
	_, _ = unix.Poll(fds, timeoutMs)
}

// reap takes completed transmit frames back.
func (s *Socket) reap() {
	prod := atomic.LoadUint32(s.comp.producer)
	cons := atomic.LoadUint32(s.comp.consumer)
	for i := cons; i != prod; i++ {
		s.txFree = append(s.txFree, *s.comp.addr(i))
	}
	atomic.StoreUint32(s.comp.consumer, prod)
}

// Queue has fill write a frame into a free transmit buffer and return
// its length, and queues it for the next Flush. It reports false, without
// calling fill, when every buffer is in flight (Flush and try again).
func (s *Socket) Queue(fill func(frame []byte) int) bool {
	s.txMu.Lock()
	defer s.txMu.Unlock()
	if len(s.txFree) == 0 {
		s.reap()
		if len(s.txFree) == 0 {
			return false
		}
	}
	if s.txQueue+(atomic.LoadUint32(s.tx.producer)-atomic.LoadUint32(s.tx.consumer)) >= ringSize {
		return false
	}
	a := s.txFree[len(s.txFree)-1]
	s.txFree = s.txFree[:len(s.txFree)-1]
	n := fill(s.umem[a : a+FrameSize])
	d := s.tx.desc(atomic.LoadUint32(s.tx.producer) + s.txQueue)
	d.Addr, d.Len, d.Options = a, uint32(n), 0
	s.txQueue++
	return true
}

// Flush publishes the queued frames and kicks the kernel when it asks
// for it (always in copy mode). It reports how many frames went out.
func (s *Socket) Flush() int {
	s.txMu.Lock()
	defer s.txMu.Unlock()
	n := s.txQueue
	if n > 0 {
		atomic.StoreUint32(s.tx.producer, atomic.LoadUint32(s.tx.producer)+n)
		s.txQueue = 0
	}
	if n > 0 || s.tx.needWakeup() {
		if s.tx.needWakeup() || !s.ZeroCopy {
			_, _, _ = unix.Syscall6(unix.SYS_SENDTO, uintptr(s.fd), 0, 0, unix.MSG_DONTWAIT, 0, 0)
		}
	}
	s.reap()
	return int(n)
}

// Close releases the rings, the UMEM and the socket. Nothing may use the
// socket any more.
func (s *Socket) Close() {
	for _, r := range []*ring{&s.rx, &s.tx, &s.fill, &s.comp} {
		if r.mem != nil {
			_ = unix.Munmap(r.mem)
			r.mem = nil
		}
	}
	if s.fd > 0 {
		_ = unix.Close(s.fd)
		s.fd = -1
	}
	if s.umem != nil {
		_ = unix.Munmap(s.umem)
		s.umem = nil
	}
}
