package profile

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func addrs(ss ...string) []netip.Addr {
	out := make([]netip.Addr, 0, len(ss))
	for _, s := range ss {
		out = append(out, netip.MustParseAddr(s))
	}
	return out
}

func TestAllocateIPsSequential(t *testing.T) {
	got, err := AllocateIPs("10.0.1.0/24", "10.0.1.100", 3, nil)
	require.NoError(t, err)
	require.Equal(t, addrs("10.0.1.100", "10.0.1.101", "10.0.1.102"), got)
}

func TestAllocateIPsSkipsNetworkBroadcastAndExcluded(t *testing.T) {
	got, err := AllocateIPs("10.0.1.0/29", "10.0.1.0", 4, addrs("10.0.1.1", "10.0.1.3"))
	require.NoError(t, err)
	// .0 network, .1 and .3 excluded, .7 broadcast never reached
	require.Equal(t, addrs("10.0.1.2", "10.0.1.4", "10.0.1.5", "10.0.1.6"), got)
}

func TestAllocateIPsReportsShortfall(t *testing.T) {
	_, err := AllocateIPs("10.0.1.0/29", "10.0.1.2", 10, addrs("10.0.1.1"))
	require.EqualError(t, err, "10.0.1.0/29 from 10.0.1.2 has only 5 usable IPs, need 10 (short by 5)")
}

func TestAllocateIPsSlash31And32UseEveryAddress(t *testing.T) {
	got, err := AllocateIPs("10.0.1.0/31", "10.0.1.0", 2, nil)
	require.NoError(t, err)
	require.Equal(t, addrs("10.0.1.0", "10.0.1.1"), got)

	got, err = AllocateIPs("10.0.1.9/32", "10.0.1.9", 1, nil)
	require.NoError(t, err)
	require.Equal(t, addrs("10.0.1.9"), got)
}

func TestAllocateIPsTopOfAddressSpaceTerminates(t *testing.T) {
	got, err := AllocateIPs("255.255.255.254/31", "255.255.255.254", 2, nil)
	require.NoError(t, err)
	require.Equal(t, addrs("255.255.255.254", "255.255.255.255"), got)
}

func TestAllocateIPsRejectsBadInput(t *testing.T) {
	_, err := AllocateIPs("10.0.1.0", "10.0.1.2", 1, nil)
	require.ErrorContains(t, err, "not an IPv4 CIDR")
	_, err = AllocateIPs("10.0.1.0/24", "10.0.2.2", 1, nil)
	require.EqualError(t, err, "start IP 10.0.2.2 is outside 10.0.1.0/24")
	_, err = AllocateIPs("fd00::/64", "fd00::1", 1, nil)
	require.ErrorContains(t, err, "not an IPv4 CIDR")
}

// A mistyped gNB count must give the shortfall error, not try to allocate
// room for billions of addresses up front.
func TestAllocateIPsHugeCountDoesNotPreallocate(t *testing.T) {
	_, err := AllocateIPs("10.0.1.0/30", "10.0.1.1", 1<<40, nil)
	require.EqualError(t, err, "10.0.1.0/30 from 10.0.1.1 has only 2 usable IPs, need 1099511627776 (short by 1099511627774)")
}
