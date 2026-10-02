package gnb

import (
	"net"
	"os"
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
	require.Less(t, time.Since(start), time.Second, "SO_RCVTIMEO should end the read")
}
