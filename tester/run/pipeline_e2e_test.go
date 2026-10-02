package run

import (
	"encoding/json"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	loggergo "github.com/Alonza0314/logger-go/v2"
	"github.com/stretchr/testify/require"

	"tester/dataplane"
	"tester/gnb"
	"tester/internal/fakecore"
	"tester/metrics"
	"tester/profile"
)

// amfDialer connects every gNB to the same fake AMF over a fresh pipe;
// IPs listed in refuse fail to connect.
type amfDialer struct {
	amf    *fakecore.AMF
	refuse map[string]bool
	ends   *sync.Map // local IP -> AMF side *fakecore.PipeEnd, if set
}

func (d amfDialer) Dial(localIP, _ string, _ int, _ time.Duration) (gnb.Conn, error) {
	if d.refuse[localIP] {
		return nil, errors.New("connection refused")
	}
	g, a := fakecore.Pipe()
	if d.ends != nil {
		d.ends.Store(localIP, a)
	}
	go func() { _ = d.amf.Serve(a) }()
	return g, nil
}

func newFakeAMF(behavior func(supi string) fakecore.Behavior) *fakecore.AMF {
	return &fakecore.AMF{
		Subscriber: fakecore.Subscriber{Mcc: "208", Mnc: "93",
			Key: "8baf473f2f8fd09487cccbd7097c6862", Opc: "8e27b6af0e692e750f32667a3b14605d",
			Amf: "8000", Sqn: "000000000023"},
		Behavior: behavior,
		UpfIP:    netip.MustParseAddr("10.0.1.5"),
	}
}

func e2eProfile() profile.Profile {
	p := testProfile()
	p.Rates.Registration = profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 2000, Retries: 1}
	p.Rates.Pdu = profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 2000, Retries: 1}
	p.Rates.Deregistration = profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 2000, Retries: 1}
	return p
}

func newE2EController(d amfDialer) (*Controller, *fakeAddrs) {
	c, addrs, _ := newE2EControllerWithDataplane(d, nil)
	return c, addrs
}

// newE2EControllerWithDataplane also returns the fake data plane; startErr
// makes its Start fail.
func newE2EControllerWithDataplane(d amfDialer, startErr error) (*Controller, *fakeAddrs, *fakeDataplane) {
	lg := loggergo.NewLogger("", true)
	lg.SetLevel("error")
	addrs := &fakeAddrs{}
	dp := &fakeDataplane{ues: map[int]fakeFlow{}, startErr: startErr}
	c := NewController(Deps{Addrs: addrs, Dialer: d, Log: lg.WithTags("TEST"), NewID: func() string { return "e2e" },
		NewDataplane: func(cfg dataplane.Config) Dataplane { dp.cfg = cfg; return dp }})
	return c, addrs, dp
}

func stopAndWait(t *testing.T, c *Controller) Snapshot {
	t.Helper()
	_, err := c.Stop()
	require.NoError(t, err)
	return waitState(t, c, StateStopped)
}

func TestRunRegistersAndEstablishesEveryUe(t *testing.T) {
	amf := newFakeAMF(nil)
	c, addrs := newE2EController(amfDialer{amf: amf})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)

	snap := waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })
	require.Equal(t, StateRunning, snap.State)
	for _, st := range []metrics.StageSnapshot{snap.Registration, snap.Pdu} {
		require.Equal(t, int64(10), st.Accepted, st.Name)
		require.True(t, st.Done, st.Name)
		require.Greater(t, st.AvgMs, 0.0, st.Name)
	}
	require.Equal(t, []int{4, 4, 2}, []int{snap.Gnbs[0].Established, snap.Gnbs[1].Established, snap.Gnbs[2].Established})
	require.Equal(t, []int{4, 4, 2}, []int{snap.Gnbs[0].Registered, snap.Gnbs[1].Registered, snap.Gnbs[2].Registered})
	require.Empty(t, snap.FailedUes)
	require.Eventually(t, func() bool { return len(amf.SetupResponses()) == 10 }, time.Second, 5*time.Millisecond)

	snap = stopAndWait(t, c)
	require.Equal(t, UeSummary{Deregistered: 10}, snap.Ues)
	present, _ := addrs.snapshot()
	require.Empty(t, present)
}

func TestRegistrationRejectIsRetriedThenListed(t *testing.T) {
	amf := newFakeAMF(func(supi string) fakecore.Behavior {
		if supi == "imsi-208930000000003" {
			return fakecore.Behavior{RejectRegistration: 7}
		}
		return fakecore.Behavior{}
	})
	c, _ := newE2EController(amfDialer{amf: amf})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)

	snap := waitFor(t, c, "pdu stage done", func(s Snapshot) bool { return s.Pdu.Done })
	require.Equal(t, int64(9), snap.Registration.Accepted)
	require.Equal(t, int64(1), snap.Registration.Rejected)
	require.Equal(t, int64(1), snap.Registration.Retries)
	require.Equal(t, []metrics.CauseCount{{Cause: "5gmm(7)", Count: 1}}, snap.Registration.Causes)
	require.Equal(t, int64(1), snap.Pdu.Skipped, "a UE that never registered skips the PDU stage")
	require.Equal(t, []UeFailure{{Supi: "imsi-208930000000003", Gnb: "gNB-1", Stage: "registration", Cause: "5gmm(7)", Attempts: 2}}, snap.FailedUes)
	require.Equal(t, UeSummary{Established: 9, Failed: 1}, snap.Ues)
	stopAndWait(t, c)
}

func TestPduRejectIsListed(t *testing.T) {
	amf := newFakeAMF(func(supi string) fakecore.Behavior {
		if supi == "imsi-208930000000010" {
			return fakecore.Behavior{RejectPdu: 27}
		}
		return fakecore.Behavior{}
	})
	c, _ := newE2EController(amfDialer{amf: amf})
	p := e2eProfile()
	p.Rates.Pdu.Retries = 0
	_, err := c.Start(p)
	require.NoError(t, err)
	snap := waitFor(t, c, "pdu stage done", func(s Snapshot) bool { return s.Pdu.Done })
	require.Equal(t, []UeFailure{{Supi: "imsi-208930000000010", Gnb: "gNB-3", Stage: "pdu", Cause: "5gsm(27)", Attempts: 1}}, snap.FailedUes)
	require.Equal(t, 1, snap.Gnbs[2].Established)
	require.Equal(t, 2, snap.Gnbs[2].Registered)
	stopAndWait(t, c)
}

func TestUesOfFailedGnbAreSkipped(t *testing.T) {
	c, _ := newE2EController(amfDialer{amf: newFakeAMF(nil), refuse: map[string]bool{"10.0.1.12": true}})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	snap := waitFor(t, c, "pdu stage done", func(s Snapshot) bool { return s.Pdu.Done })
	require.Equal(t, GnbFailed, snap.Gnbs[2].State)
	require.Equal(t, int64(2), snap.Registration.Skipped)
	require.Equal(t, int64(8), snap.Registration.Accepted)
	require.Equal(t, UeSummary{Established: 8, Skipped: 2}, snap.Ues)
	stopAndWait(t, c)
}

func TestStopDuringRegistrationCancelsQueuedUes(t *testing.T) {
	c, _ := newE2EController(amfDialer{amf: newFakeAMF(nil)})
	p := e2eProfile()
	p.Rates.Registration.RatePerSec = 5 // 10 UEs would take ~2 s
	_, err := c.Start(p)
	require.NoError(t, err)
	waitFor(t, c, "one UE established", func(s Snapshot) bool { return s.Ues.Established >= 1 })

	snap := stopAndWait(t, c)
	require.True(t, snap.Registration.Done, "stopped stage must be complete")
	require.True(t, snap.Pdu.Done)
	require.Positive(t, snap.Ues.Cancelled)
	require.Equal(t, int64(snap.Ues.Cancelled), snap.Registration.Skipped)
	require.Equal(t, 10, snap.Ues.Deregistered+snap.Ues.Cancelled, "UEs that got through deregister")
	require.True(t, snap.Deregistration.Done)
	frozen := snap.Registration.TotalTimeMs
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, frozen, c.Snapshot().Registration.TotalTimeMs, "total time stops at the last finish")
}

// A run where every UE succeeds must still send "failedUes": [] — the
// Run page reads failedUes.length, and null blanked it.
func TestSnapshotJSONHasEmptyFailedUesNotNull(t *testing.T) {
	c, _ := newE2EController(amfDialer{amf: newFakeAMF(nil)})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })
	raw, err := json.Marshal(c.Snapshot())
	require.NoError(t, err)
	require.Contains(t, string(raw), `"failedUes":[]`)
	stopAndWait(t, c)
}

func TestEstablishedUesStartTrafficWithTheirTunnel(t *testing.T) {
	c, _, dp := newE2EControllerWithDataplane(amfDialer{amf: newFakeAMF(nil)}, nil)
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })

	require.Equal(t, uint16(9200), dp.cfg.Port)
	require.Equal(t, trafficStartDelay, dp.cfg.StartDelay, "traffic waits for the UPF to learn the DL tunnel")
	require.Equal(t, 1400, dp.cfg.PacketSize)
	require.Equal(t, 5e6, dp.cfg.DlBps)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.2.10"), netip.MustParseAddr("10.0.2.11"), netip.MustParseAddr("10.0.2.12")}, dp.cfg.GnbN3IPs)
	flows := dp.flows()
	require.Len(t, flows, 10)
	teids := map[uint32]bool{}
	for ue, f := range flows {
		require.Equal(t, ue/4, f.gnb, "UE %d", ue)
		require.Equal(t, netip.MustParseAddrPort("10.0.1.5:2152"), f.upf, "UPF N3 from the PDU Session Resource Setup")
		require.Equal(t, uint32(0x1000), f.ulTeid&0xffffff00, "UL TEID from the AMF's transfer")
		teids[f.dlTeid] = true
	}
	require.Len(t, teids, 10)
	stopAndWait(t, c)
	require.True(t, dp.stopped)
}

func TestDataplaneStartFailureFailsTheRunAndRollsBack(t *testing.T) {
	c, addrs, _ := newE2EControllerWithDataplane(amfDialer{amf: newFakeAMF(nil)}, errors.New("bind gNB-1 N3 10.0.2.10:2152: address already in use"))
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	snap := waitState(t, c, StateFailed)
	require.Contains(t, snap.Error, "start data plane: bind gNB-1 N3")
	present, _ := addrs.snapshot()
	require.Empty(t, present)
	require.Equal(t, int64(0), snap.N2.Attempted)
}

func TestStopDeregistersEveryUeThenClosesN2(t *testing.T) {
	amf := newFakeAMF(nil)
	c, addrs := newE2EController(amfDialer{amf: amf})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })

	snap := stopAndWait(t, c)
	require.Equal(t, int64(10), snap.Deregistration.Accepted)
	require.True(t, snap.Deregistration.Done)
	require.Greater(t, snap.Deregistration.AvgMs, 0.0)
	require.Equal(t, UeSummary{Deregistered: 10}, snap.Ues)
	require.Equal(t, 10, amf.Deregistrations())
	require.Equal(t, int64(3), snap.N2Release.Accepted)
	require.True(t, snap.N2Release.Done)
	require.Equal(t, "user", snap.StopReason)
	for _, g := range snap.Gnbs {
		require.Equal(t, GnbClosed, g.State)
	}
	present, _ := addrs.snapshot()
	require.Empty(t, present)
}

func TestStopDeregistersRegisteredButNotEstablishedUes(t *testing.T) {
	amf := newFakeAMF(func(string) fakecore.Behavior { return fakecore.Behavior{RejectPdu: 27} })
	c, _ := newE2EController(amfDialer{amf: amf})
	p := e2eProfile()
	p.Rates.Pdu.Retries = 0
	_, err := c.Start(p)
	require.NoError(t, err)
	waitFor(t, c, "every PDU rejected", func(s Snapshot) bool { return s.Pdu.Done })

	snap := stopAndWait(t, c)
	require.Equal(t, int64(10), snap.Deregistration.Accepted, "registered UEs deregister even without a PDU session")
	require.Equal(t, 10, amf.Deregistrations())
}

func TestUesThatNeverRegisteredAreSkippedAtCleanup(t *testing.T) {
	amf := newFakeAMF(func(supi string) fakecore.Behavior {
		if supi == "imsi-208930000000003" {
			return fakecore.Behavior{RejectRegistration: 7}
		}
		return fakecore.Behavior{}
	})
	c, _ := newE2EController(amfDialer{amf: amf})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	waitFor(t, c, "pdu stage done", func(s Snapshot) bool { return s.Pdu.Done })

	snap := stopAndWait(t, c)
	require.Equal(t, int64(9), snap.Deregistration.Accepted)
	require.Equal(t, int64(1), snap.Deregistration.Skipped)
	require.True(t, snap.Deregistration.Done)
	require.Equal(t, UeSummary{Deregistered: 9, Failed: 1}, snap.Ues)
}

func TestDeregistrationTimeoutIsCountedAndListed(t *testing.T) {
	amf := newFakeAMF(func(string) fakecore.Behavior { return fakecore.Behavior{IgnoreDeregistration: true} })
	c, _ := newE2EController(amfDialer{amf: amf})
	p := e2eProfile()
	p.Rates.Deregistration = profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 100, Retries: 0}
	_, err := c.Start(p)
	require.NoError(t, err)
	waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })

	snap := stopAndWait(t, c)
	require.Equal(t, int64(10), snap.Deregistration.TimedOut)
	require.Equal(t, 10, snap.Ues.Failed)
	require.Len(t, snap.FailedUes, 10)
	require.Equal(t, "deregistration", snap.FailedUes[0].Stage)
}

func TestUesOfALostGnbAreSkippedAtCleanup(t *testing.T) {
	ends := &sync.Map{}
	c, _ := newE2EController(amfDialer{amf: newFakeAMF(nil), ends: ends})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })
	end, ok := ends.Load("10.0.1.10") // gNB-1's N2 IP
	require.True(t, ok)
	_ = end.(*fakecore.PipeEnd).Close()
	waitFor(t, c, "gNB-1 lost", func(s Snapshot) bool { return s.Gnbs[0].State == GnbLost })

	snap := stopAndWait(t, c)
	require.Equal(t, int64(4), snap.Deregistration.Skipped)
	require.Equal(t, int64(6), snap.Deregistration.Accepted)
	require.Equal(t, int64(2), snap.N2Release.Accepted)
	require.Equal(t, int64(1), snap.N2Release.Skipped)
}

func TestSecondStopAbortsCleanup(t *testing.T) {
	amf := newFakeAMF(func(string) fakecore.Behavior { return fakecore.Behavior{IgnoreDeregistration: true} })
	c, addrs := newE2EController(amfDialer{amf: amf})
	p := e2eProfile()
	p.Rates.Deregistration = profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 5, TimeoutMs: 60000, Retries: 0}
	_, err := c.Start(p)
	require.NoError(t, err)
	waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })
	_, err = c.Stop()
	require.NoError(t, err)
	waitFor(t, c, "deregistering", func(s Snapshot) bool { return s.Deregistration.InFlight == 5 })

	_, err = c.Stop() // skip the rest of cleanup
	require.NoError(t, err)
	snap := waitState(t, c, StateStopped)
	require.True(t, snap.Deregistration.Done)
	require.Equal(t, int64(5), snap.Deregistration.Skipped, "the queued half was never sent")
	require.True(t, snap.N2Release.Done)
	present, _ := addrs.snapshot()
	require.Empty(t, present)
	_, err = c.Stop()
	require.ErrorIs(t, err, ErrNotRunning)
}

func TestMaxDurationStopsTheRun(t *testing.T) {
	c, _ := newE2EController(amfDialer{amf: newFakeAMF(nil)})
	c.deps.MaxDurationUnit = time.Millisecond
	p := e2eProfile()
	p.Traffic.MaxDurationMin = 300 // 300 ms with the test unit
	_, err := c.Start(p)
	require.NoError(t, err)
	snap := waitState(t, c, StateStopped)
	require.Equal(t, "maxDuration", snap.StopReason)
	require.Equal(t, int64(10), snap.Deregistration.Accepted)
}
