package run

import (
	"encoding/json"
	"errors"
	"net/netip"
	"testing"
	"time"

	loggergo "github.com/Alonza0314/logger-go/v2"
	"github.com/stretchr/testify/require"

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
}

func (d amfDialer) Dial(localIP, _ string, _ int, _ time.Duration) (gnb.Conn, error) {
	if d.refuse[localIP] {
		return nil, errors.New("connection refused")
	}
	g, a := fakecore.Pipe()
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
	return p
}

func newE2EController(d amfDialer) (*Controller, *fakeAddrs) {
	lg := loggergo.NewLogger("", true)
	lg.SetLevel("error")
	addrs := &fakeAddrs{}
	return NewController(Deps{Addrs: addrs, Dialer: d, Log: lg.WithTags("TEST"), NewID: func() string { return "e2e" }}), addrs
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
	require.Equal(t, UeSummary{Established: 10}, snap.Ues)
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
	require.Equal(t, 10, snap.Ues.Established+snap.Ues.Cancelled)
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
