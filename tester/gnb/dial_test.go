package gnb

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/free5gc/sctp"
	"github.com/stretchr/testify/require"
)

// Needs the sctp kernel module: FRU_TESTER_SCTP=1 go test ./gnb/ -run SCTP
func TestSCTPDialerLoopback(t *testing.T) {
	if os.Getenv("FRU_TESTER_SCTP") != "1" {
		t.Skip("set FRU_TESTER_SCTP=1 (and `sudo modprobe sctp`) to run")
	}
	ln, err := sctp.ListenSCTP("sctp", &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, Port: 0})
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*sctp.SCTPAddr).Port

	go func() {
		c, err := ln.AcceptSCTP(-1)
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		time.Sleep(2 * time.Second) // never answer
	}()

	conn, err := SCTPDialer{}.Dial("127.0.0.1", "127.0.0.1", port, 300*time.Millisecond)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	start := time.Now()
	_, err = conn.Read(make([]byte, 16))
	require.Error(t, err)
	require.Less(t, time.Since(start), time.Second, "the read deadline should end the read")
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)
}

// sctpServer accepts every association on loopback and hands each to
// serve in its own goroutine.
func sctpServer(t *testing.T, serve func(*sctp.SCTPConn)) int {
	t.Helper()
	ln, err := sctp.ListenSCTP("sctp", &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, Port: 0})
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.AcceptSCTP(-1)
			if errors.Is(err, syscall.EINTR) {
				continue // a blocking accept, interrupted by a runtime signal
			}
			if err != nil {
				return
			}
			go serve(c)
		}
	}()
	return ln.Addr().(*sctp.SCTPAddr).Port
}

func needSCTP(t *testing.T) {
	if os.Getenv("FRU_TESTER_SCTP") != "1" {
		t.Skip("set FRU_TESTER_SCTP=1 (and `sudo modprobe sctp`) to run")
	}
}

// Messages go out with the NGAP PPID and come back whole; after
// NewAssociation a read outlives the dial timeout.
func TestSCTPDialerRoundTripAndNoReadTimeoutAfterSetup(t *testing.T) {
	needSCTP(t)
	ppid := make(chan uint32, 1)
	port := sctpServer(t, func(c *sctp.SCTPConn) {
		defer func() { _ = c.Close() }()
		_ = c.SubscribeEvents(sctp.SCTP_EVENT_DATA_IO)
		buf := make([]byte, 4096)
		n, info, _, err := c.SCTPRead(buf)
		if err != nil {
			return
		}
		if info != nil {
			ppid <- info.PPID
		}
		_, _ = c.Write(buf[:n]) // the "NG Setup Response"
		time.Sleep(500 * time.Millisecond)
		_, _ = c.Write([]byte("later"))
		time.Sleep(time.Second)
	})
	conn, err := SCTPDialer{}.Dial("127.0.0.1", "127.0.0.1", port, 200*time.Millisecond)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	_, err = conn.Write([]byte("hello"))
	require.NoError(t, err)
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "hello", string(buf[:n]))
	require.Equal(t, ngapPPID, <-ppid)

	NewAssociation(conn, Identity{}, netip.Addr{}, nil) // lifts the read deadline
	n, err = conn.Read(buf)
	require.NoError(t, err, "a read after setup waits longer than the dial timeout")
	require.Equal(t, "later", string(buf[:n]))
}

// An idle association waiting in Read holds no OS thread: 300 of them
// would have taken 300 with free5gc/sctp's blocking Recvmsg.
func TestSCTPDialerIdleAssociationsHoldNoThreads(t *testing.T) {
	needSCTP(t)
	hold := make(chan struct{})
	port := sctpServer(t, func(c *sctp.SCTPConn) {
		<-hold
		_ = c.Close()
	})
	defer close(hold)
	before := threads(t)
	const gnbs = 300
	var conns []Conn
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()
	for range gnbs {
		c, err := SCTPDialer{}.Dial("127.0.0.1", "127.0.0.1", port, time.Second)
		require.NoError(t, err)
		NewAssociation(c, Identity{}, netip.Addr{}, nil)
		conns = append(conns, c)
		go func() { _, _ = c.Read(make([]byte, 64)) }()
	}
	time.Sleep(300 * time.Millisecond)
	require.Less(t, threads(t)-before, 50, "idle reads must not each hold a thread")
}

func threads(t *testing.T) int {
	b, err := os.ReadFile("/proc/self/status")
	require.NoError(t, err)
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, "Threads:"); ok {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			require.NoError(t, err)
			return n
		}
	}
	t.Fatal("no Threads line")
	return 0
}
