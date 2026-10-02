package gnb

import (
	"fmt"
	"io"
	"net"
	"syscall"
	"time"

	"github.com/free5gc/sctp"
)

// ngapPPID is the SCTP payload protocol identifier for NGAP (TS 38.412).
const ngapPPID uint32 = 0x3c000000

// Conn is one gNB's N2 association.
type Conn = io.ReadWriteCloser

// Dialer opens an N2 association from localIP to the AMF. timeout bounds
// both the connect and every later blocking read on the returned Conn.
type Dialer interface {
	Dial(localIP, amfIP string, amfPort int, timeout time.Duration) (Conn, error)
}

// SCTPDialer is the real Dialer. The local port is 0 (kernel-chosen), so
// gNBs never collide on ports even when they share an IP.
type SCTPDialer struct{}

func (SCTPDialer) Dial(localIP, amfIP string, amfPort int, timeout time.Duration) (Conn, error) {
	local, err := sctpAddr(localIP, 0)
	if err != nil {
		return nil, err
	}
	remote, err := sctpAddr(amfIP, amfPort)
	if err != nil {
		return nil, err
	}
	tv := syscall.NsecToTimeval(timeout.Nanoseconds())
	cfg := sctp.SocketConfig{
		InitMsg: sctp.InitMsg{NumOstreams: sctp.SCTP_MAX_STREAM},
		// SO_SNDTIMEO bounds the blocking connect; SO_RCVTIMEO bounds
		// reads, since SCTPConn.SetReadDeadline returns EOPNOTSUPP.
		Control: func(_, _ string, c syscall.RawConn) error {
			var serr error
			if err := c.Control(func(fd uintptr) {
				if serr = syscall.SetsockoptTimeval(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &tv); serr != nil {
					return
				}
				serr = syscall.SetsockoptTimeval(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
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
	return conn, nil
}

func sctpAddr(ip string, port int) (*sctp.SCTPAddr, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, fmt.Errorf("%q is not an IP address", ip)
	}
	return &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: parsed}}, Port: port}, nil
}
