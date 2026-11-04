package xdp

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// A run with 10 000 gNBs allows 10 000 N3 addresses plus the sink; the
// map was a fixed 4096 and the 4097th Put failed with E2BIG.
func TestTargetsMapHoldsEveryGnbAndTheSink(t *testing.T) {
	require.EqualValues(t, minTargets, steeringSpec(10).Maps["targets"].MaxEntries)
	require.EqualValues(t, 10_001, steeringSpec(10_001).Maps["targets"].MaxEntries)
}

func TestSteeringAllowsTenThousandGnbs(t *testing.T) {
	ifindex := vethPair(t)
	const gnbs = 10_000
	st, err := Steer(ifindex, gnbs+1)
	require.NoError(t, err)
	defer st.Close()
	base := netip.MustParseAddr("10.1.0.0")
	addr := base
	for range gnbs {
		addr = addr.Next()
		require.NoError(t, st.Allow(addr, 2152))
	}
	require.NoError(t, st.Allow(netip.MustParseAddr("10.98.0.1"), 9200))
}
