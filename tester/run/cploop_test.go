package run

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tester/internal/fakecore"
	"tester/profile"
)

// cpProfile: 10 UEs fill 3 gNBs 4/4/2; the first throughput carry
// traffic and the rest cycle (with 4: all of gNB-2's and gNB-3's).
func cpProfile(throughput int) profile.Profile {
	p := e2eProfile()
	p.Scale.ThroughputUeCount = &throughput
	p.Rates.CpLoop = profile.CpLoopRate{RatePerSec: 1000, MaxInFlight: 10, TimeoutMs: 2000, HoldMs: 10}
	return p
}

func TestControlPlaneUesCycleUntilStop(t *testing.T) {
	amf := newFakeAMF(nil)
	c, _, dp := newE2EControllerWithDataplane(amfDialer{amf: amf}, nil)
	_, err := c.Start(cpProfile(4))
	require.NoError(t, err)

	snap := waitFor(t, c, "30 control-plane cycles", func(s Snapshot) bool { return s.CpLoop.Cycles >= 30 })
	require.Equal(t, CpRunning, snap.CpLoop.State)
	require.Equal(t, 6, snap.CpLoop.Ues)
	require.NotNil(t, snap.CpLoop.StartedAt)
	// only the throughput UEs are in the registration and PDU stages
	for _, st := range []int64{snap.Registration.Accepted, snap.Pdu.Accepted} {
		require.Equal(t, int64(4), st)
	}
	require.Equal(t, 4, snap.Registration.Expected)
	require.True(t, snap.Pdu.Done)
	require.Equal(t, 4, snap.Ues.Established)
	require.Len(t, dp.flows(), 4, "control-plane UEs carry no traffic")
	require.GreaterOrEqual(t, snap.CpLoop.Registration.Accepted, snap.CpLoop.Cycles)
	require.Greater(t, snap.CpLoop.Registration.P50Ms, 0.0)
	require.Greater(t, snap.CpLoop.Deregistration.P50Ms, 0.0)
	require.Empty(t, snap.FailedUes)
	require.Equal(t, []int{0, 4, 2}, []int{snap.Gnbs[0].CpUeCount, snap.Gnbs[1].CpUeCount, snap.Gnbs[2].CpUeCount})

	snap = stopAndWait(t, c)
	require.Equal(t, CpStopped, snap.CpLoop.State)
	require.Equal(t, UeSummary{Deregistered: 10}, snap.Ues, "every UE ends deregistered")
	require.Zero(t, snap.CpLoop.Registered)
	// cleanup counts the throughput UEs and the control-plane UEs apart
	require.Equal(t, 4, snap.Deregistration.Expected)
	require.Equal(t, int64(4), snap.Deregistration.Accepted)
	require.True(t, snap.Deregistration.Done)
	cleanup := snap.CpLoop.Cleanup
	require.Equal(t, 6, cleanup.Expected)
	require.True(t, cleanup.Done)
	require.Equal(t, int64(6), cleanup.Accepted+cleanup.Skipped, "registered at Stop: deregistered; between cycles: not started")
	// one deregistration per cycle, plus cleanup's
	require.Equal(t, int(snap.CpLoop.Deregistration.Accepted+snap.Deregistration.Accepted+cleanup.Accepted), amf.Deregistrations())
	cycles := snap.CpLoop.Cycles
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, cycles, c.Snapshot().CpLoop.Cycles, "the loop stopped")
}

// The loop waits until every throughput UE has a final PDU outcome.
func TestControlPlaneLoopWaitsForThroughputUes(t *testing.T) {
	amf := newFakeAMF(func(supi string) fakecore.Behavior {
		return fakecore.Behavior{IgnorePduRequest: supi == "imsi-208930000000001"}
	})
	c, _ := newE2EController(amfDialer{amf: amf})
	p := cpProfile(4)
	p.Rates.Pdu.TimeoutMs, p.Rates.Pdu.Retries = 400, 0
	_, err := c.Start(p)
	require.NoError(t, err)

	snap := waitFor(t, c, "3 UEs established", func(s Snapshot) bool { return s.Ues.Established == 3 })
	require.Equal(t, CpWaiting, snap.CpLoop.State)
	require.Zero(t, snap.CpLoop.Registration.Attempted)

	snap = waitFor(t, c, "the loop running", func(s Snapshot) bool { return s.CpLoop.Cycles > 0 })
	require.Equal(t, int64(1), snap.Pdu.TimedOut)
	stopAndWait(t, c)
}

// A failed cycle is counted with its cause, not listed as a failed UE,
// and the UE keeps cycling.
func TestControlPlaneFailuresAreCountedNotListed(t *testing.T) {
	amf := newFakeAMF(func(supi string) fakecore.Behavior {
		if supi == "imsi-208930000000010" {
			return fakecore.Behavior{RejectRegistration: 111}
		}
		return fakecore.Behavior{}
	})
	c, _ := newE2EController(amfDialer{amf: amf})
	_, err := c.Start(cpProfile(4))
	require.NoError(t, err)

	snap := waitFor(t, c, "UE 10 rejected three times while others cycle", func(s Snapshot) bool {
		return s.CpLoop.Registration.Rejected >= 3 && s.CpLoop.Cycles > 0
	})
	require.Equal(t, "5gmm(111)", snap.CpLoop.Registration.Causes[0].Cause)
	require.Empty(t, snap.FailedUes)
	stopAndWait(t, c)
}

// With no throughput UEs the loop starts as soon as N2 is up.
func TestEveryUeInTheControlPlaneLoop(t *testing.T) {
	amf := newFakeAMF(nil)
	c, _, dp := newE2EControllerWithDataplane(amfDialer{amf: amf}, nil)
	_, err := c.Start(cpProfile(0))
	require.NoError(t, err)
	snap := waitFor(t, c, "cycles", func(s Snapshot) bool { return s.CpLoop.Cycles >= 10 })
	require.True(t, snap.Registration.Done)
	require.True(t, snap.Pdu.Done)
	require.Empty(t, dp.flows())
	snap = stopAndWait(t, c)
	require.Equal(t, UeSummary{Deregistered: 10}, snap.Ues)
}

func TestNoControlPlaneUesMeansTheLoopIsOff(t *testing.T) {
	c, _ := newE2EController(amfDialer{amf: newFakeAMF(nil)})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	snap := waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })
	require.Equal(t, CpOff, snap.CpLoop.State)
	require.Empty(t, snap.CpLoop.Series)
	snap = stopAndWait(t, c)
	require.Equal(t, CpOff, snap.CpLoop.State)
}
