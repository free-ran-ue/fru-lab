package xdp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
)

// MaxQueues is the most queues of one interface Steering can serve.
const MaxQueues = 256

// Steering is the XDP program on one interface. It sends IPv4/UDP packets
// addressed to an allowed (address, port) to the AF_XDP socket of the
// queue they arrived on, and lets everything else (SCTP, ARP, ICMP, other
// UDP) go on to the kernel. A queue without a socket also lets them pass.
type Steering struct {
	coll *ebpf.Collection
	link link.Link
	// Generic is set when the driver could not run the program natively
	// and it runs in the kernel's generic (SKB) mode: slower, and only
	// copy-mode sockets work.
	Generic bool
}

// steeringProgram checks Ethernet → IPv4 without options → unfragmented
// UDP, looks up (destination address, destination port) in "targets" and
// redirects a match to xsks[rx_queue_index], passing it to the kernel if
// that queue has no socket.
func steeringProgram() asm.Instructions {
	const (
		ethLen  = 14
		minLen  = ethLen + 20 + 8
		xdpPass = 2
	)
	return asm.Instructions{
		asm.Mov.Reg(asm.R6, asm.R1),                   // r6 = ctx
		asm.LoadMem(asm.R2, asm.R6, 0, asm.Word),      // r2 = data
		asm.LoadMem(asm.R3, asm.R6, 4, asm.Word),      // r3 = data_end
		asm.Mov.Reg(asm.R4, asm.R2),                   //
		asm.Add.Imm(asm.R4, minLen),                   //
		asm.JGT.Reg(asm.R4, asm.R3, "pass"),           // too short
		asm.LoadMem(asm.R4, asm.R2, 12, asm.Half),     // ethertype
		asm.JNE.Imm(asm.R4, 0x0008, "pass"),           // 0x0800 read little-endian
		asm.LoadMem(asm.R4, asm.R2, ethLen, asm.Byte), // version and IHL
		asm.JNE.Imm(asm.R4, 0x45, "pass"),             //
		asm.LoadMem(asm.R4, asm.R2, ethLen+9, asm.Byte),
		asm.JNE.Imm(asm.R4, 17, "pass"), // UDP
		asm.LoadMem(asm.R4, asm.R2, ethLen+6, asm.Half),
		asm.And.Imm(asm.R4, 0xff3f),    // MF and fragment offset (big-endian 0x3fff)
		asm.JNE.Imm(asm.R4, 0, "pass"), // a fragment
		asm.LoadMem(asm.R4, asm.R2, ethLen+16, asm.Word),
		asm.StoreMem(asm.RFP, -8, asm.R4, asm.Word), // key: destination address
		asm.LoadMem(asm.R4, asm.R2, ethLen+20+2, asm.Half),
		asm.StoreMem(asm.RFP, -4, asm.R4, asm.Half), // destination port
		asm.StoreImm(asm.RFP, -2, 0, asm.Half),      // padding
		asm.LoadMapPtr(asm.R1, 0).WithReference("targets"),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, -8),
		asm.FnMapLookupElem.Call(),
		asm.JEq.Imm(asm.R0, 0, "pass"),
		asm.LoadMapPtr(asm.R1, 0).WithReference("xsks"),
		asm.LoadMem(asm.R2, asm.R6, 16, asm.Word), // rx_queue_index
		asm.Mov.Imm(asm.R3, xdpPass),              // no socket on the queue: pass
		asm.FnRedirectMap.Call(),
		asm.Return(),
		asm.Mov.Imm(asm.R0, xdpPass).WithSymbol("pass"),
		asm.Return(),
	}
}

// Steer loads the program onto ifindex, natively if the driver can,
// otherwise in generic mode.
func Steer(ifindex int) (*Steering, error) {
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("lift the memlock limit: %w", err)
	}
	spec := &ebpf.CollectionSpec{
		Maps: map[string]*ebpf.MapSpec{
			"targets": {Type: ebpf.Hash, KeySize: 8, ValueSize: 1, MaxEntries: 4096},
			"xsks":    {Type: ebpf.XSKMap, KeySize: 4, ValueSize: 4, MaxEntries: MaxQueues},
		},
		Programs: map[string]*ebpf.ProgramSpec{
			"steer": {Type: ebpf.XDP, License: "Dual MIT/GPL", Instructions: steeringProgram()},
		},
	}
	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		return nil, fmt.Errorf("load the XDP program: %w", err)
	}
	s := &Steering{coll: coll}
	var errs []error
	for _, mode := range []link.XDPAttachFlags{link.XDPDriverMode, link.XDPGenericMode} {
		l, err := link.AttachXDP(link.XDPOptions{Program: coll.Programs["steer"], Interface: ifindex, Flags: mode})
		if err == nil {
			s.link, s.Generic = l, mode == link.XDPGenericMode
			return s, nil
		}
		errs = append(errs, err)
	}
	coll.Close()
	return nil, fmt.Errorf("attach the XDP program: %w", errors.Join(errs...))
}

// targetKey is the program's lookup key: the address and port as they
// sit in the packet, then two bytes of padding.
func targetKey(addr netip.Addr, port uint16) [8]byte {
	var k [8]byte
	a := addr.As4()
	copy(k[:4], a[:])
	binary.BigEndian.PutUint16(k[4:6], port)
	return k
}

// Allow sends UDP to addr:port to the sockets.
func (s *Steering) Allow(addr netip.Addr, port uint16) error {
	return s.coll.Maps["targets"].Put(targetKey(addr, port), uint8(1))
}

// Register puts sock on queue.
func (s *Steering) Register(queue int, sock *Socket) error {
	return s.coll.Maps["xsks"].Put(uint32(queue), uint32(sock.FD()))
}

// Close detaches the program.
func (s *Steering) Close() {
	if s.link != nil {
		_ = s.link.Close()
	}
	s.coll.Close()
}
