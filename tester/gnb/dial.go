package gnb

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/free5gc/sctp"
	"golang.org/x/sys/unix"
)

// ngapPPID is the SCTP payload protocol identifier for NGAP (TS 38.412).
const ngapPPID uint32 = 0x3c000000

// Conn is one gNB's N2 association.
type Conn = io.ReadWriteCloser

// Dialer opens an N2 association from localIP to the AMF. timeout bounds
// the connect, every write, and reads until the Conn is handed to
// NewAssociation (the NG Setup answer); after that a read waits for as
// long as the association lives.
type Dialer interface {
	Dial(localIP, amfIP string, amfPort int, timeout time.Duration) (Conn, error)
}

// SCTPDialer is the real Dialer. The local port is 0 (kernel-chosen), so
// gNBs never collide on ports even when they share an IP.
//
// free5gc/sctp reads and writes with blocking syscalls, so each gNB idle
// in Read would hold an OS thread: 10 000 gNBs passed Go's 10 000-thread
// limit and the tester died with "thread exhaustion". Once connected,
// the socket is made non-blocking and read and written through Go's
// poller instead (pollConn), and an idle gNB holds no thread.
type SCTPDialer struct{}

// dialAttempts: a blocking connect with SO_SNDTIMEO set is not restarted
// after a signal (the Go runtime sends them to preempt goroutines), so
// EINTR is retried on a new socket.
const dialAttempts = 3

func (d SCTPDialer) Dial(localIP, amfIP string, amfPort int, timeout time.Duration) (Conn, error) {
	var err error
	for range dialAttempts {
		var c Conn
		if c, err = d.dial(localIP, amfIP, amfPort, timeout); !errors.Is(err, syscall.EINTR) {
			return c, err
		}
	}
	return nil, err
}

func (SCTPDialer) dial(localIP, amfIP string, amfPort int, timeout time.Duration) (Conn, error) {
	local, err := sctpAddr(localIP, 0)
	if err != nil {
		return nil, err
	}
	remote, err := sctpAddr(amfIP, amfPort)
	if err != nil {
		return nil, err
	}
	tv := syscall.NsecToTimeval(timeout.Nanoseconds())
	fd := -1
	cfg := sctp.SocketConfig{
		InitMsg: sctp.InitMsg{NumOstreams: sctp.SCTP_MAX_STREAM},
		// SO_SNDTIMEO bounds the blocking connect. The socket is
		// remembered so it can go to the poller afterwards.
		Control: func(_, _ string, c syscall.RawConn) error {
			var serr error
			if err := c.Control(func(f uintptr) {
				fd = int(f)
				serr = syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &tv)
			}); err != nil {
				return err
			}
			return serr
		},
	}
	conn, err := cfg.Dial("sctp", local, remote)
	if err != nil {
		if conn != nil {
			_ = conn.Close()
		}
		return nil, fmt.Errorf("sctp connect %s -> %s:%d: %w", localIP, amfIP, amfPort, err)
	}
	info, err := conn.GetDefaultSentParam()
	if err == nil {
		info.PPID = ngapPPID
		err = conn.SetDefaultSentParam(info)
	}
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set ngap ppid: %w", err)
	}
	if fd < 0 {
		_ = conn.Close()
		return nil, errors.New("sctp connect: the socket was not seen")
	}
	if err := unix.SetNonblock(fd, true); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("make the sctp socket non-blocking: %w", err)
	}
	// From here the *os.File owns fd: conn is dropped without Close
	// (SCTPConn has no finalizer). A plain read(2) on a one-to-one SCTP
	// socket returns one message, and write(2) sends with the default
	// PPID set above, as SCTPConn's Read and Write did. NewFile sees the
	// non-blocking fd and registers it with the poller.
	f := os.NewFile(uintptr(fd), "sctp "+localIP)
	if err := f.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("sctp socket is not pollable: %w", err)
	}
	return &pollConn{f: f, timeout: timeout}, nil
}

// pollConn is an established association read and written through Go's
// poller, so a goroutine waiting on it holds no OS thread.
type pollConn struct {
	f       *os.File
	timeout time.Duration
}

func (c *pollConn) Read(b []byte) (int, error) {
	n, err := c.f.Read(b)
	if n == 0 && err == nil {
		return 0, io.EOF
	}
	return n, err
}

// Write fails after timeout if the AMF stops taking data.
func (c *pollConn) Write(b []byte) (int, error) {
	if err := c.f.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.f.Write(b)
}

// SetReadDeadline lets NewAssociation lift the NG Setup read timeout.
func (c *pollConn) SetReadDeadline(t time.Time) error { return c.f.SetReadDeadline(t) }

// Close ends the association; a Read waiting on it returns at once.
func (c *pollConn) Close() error { return c.f.Close() }

func sctpAddr(ip string, port int) (*sctp.SCTPAddr, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, fmt.Errorf("%q is not an IP address", ip)
	}
	return &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: parsed}}, Port: port}, nil
}
