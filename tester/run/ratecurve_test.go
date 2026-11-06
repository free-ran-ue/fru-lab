package run

import (
	"testing"

	"tester/internal/fakecore"

	"github.com/stretchr/testify/require"
)

func TestRateCurveHalvesWhenFull(t *testing.T) {
	c := newRateCurve[SetupPoint](4)
	for i := 1; i <= 9; i++ {
		c.add(SetupPoint{T: float64(i), RegPerSec: float64(i)})
	}
	// 1..4 fill the curve and pair into 2-second points (1.5 3.5); 5..8
	// add 5.5 7.5, fill it again and pair into 4-second points (2.5 6.5);
	// 9 is the first second of the next one.
	require.Equal(t, []SetupPoint{{T: 4, RegPerSec: 2.5}, {T: 8, RegPerSec: 6.5}, {T: 9, RegPerSec: 9}}, c.points())
}

func TestSetupCurveCountsEveryOutcome(t *testing.T) {
	amf := newFakeAMF(func(supi string) fakecore.Behavior {
		if supi == "imsi-208930000000003" {
			return fakecore.Behavior{RejectRegistration: 7}
		}
		return fakecore.Behavior{}
	})
	c, _ := newE2EController(amfDialer{amf: amf})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)

	sum := func(s Snapshot) (reg, pdu, fail float64) {
		for _, p := range s.Setup {
			reg, pdu, fail = reg+p.RegPerSec, pdu+p.PduPerSec, fail+p.FailPerSec
		}
		return
	}
	snap := waitFor(t, c, "the setup curve to cover both stages", func(s Snapshot) bool {
		reg, pdu, _ := sum(s)
		return s.Pdu.Done && reg == 9 && pdu == 9
	})
	_, _, fail := sum(snap)
	require.Equal(t, 1.0, fail, "the rejected UE fails once, after its retry")
	require.Greater(t, snap.Setup[0].T, 0.0)
	stopAndWait(t, c)
}
