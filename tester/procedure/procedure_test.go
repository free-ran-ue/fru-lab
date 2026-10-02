package procedure

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tester/gnb"
	"tester/internal/fakecore"
	"tester/metrics"
	"tester/profile"
	"tester/ue"
)

var sub = fakecore.Subscriber{Mcc: "208", Mnc: "93",
	Key: "8baf473f2f8fd09487cccbd7097c6862", Opc: "8e27b6af0e692e750f32667a3b14605d",
	Amf: "8000", Sqn: "000000000023"}

// setup starts a fake AMF on one end of a pipe and an association on the
// other, both running until the test ends.
func setup(t *testing.T, behavior func(string) fakecore.Behavior) (*gnb.Association, *fakecore.AMF, *fakecore.PipeEnd) {
	t.Helper()
	gnbEnd, amfEnd := fakecore.Pipe()
	amf := &fakecore.AMF{Subscriber: sub, Behavior: behavior, UpfIP: netip.MustParseAddr("10.0.1.5")}
	go func() { _ = amf.Serve(amfEnd) }()
	id, err := gnb.NewIdentity(profile.GnbSpec{Name: "gNB-1", GnbID: "000314"},
		profile.GnbTemplate{Mcc: "208", Mnc: "93", Tac: "000001", Sst: 1, Sd: "010203"})
	require.NoError(t, err)
	assoc := gnb.NewAssociation(gnbEnd, id, netip.MustParseAddr("10.0.1.100"), &gnb.TeidAllocator{})
	go func() { _ = assoc.Run() }()
	t.Cleanup(func() { _ = gnbEnd.Close() })
	return assoc, amf, amfEnd
}

func newUE(t *testing.T, msin string) *ue.UE {
	t.Helper()
	u, err := ue.New(ue.Config{Supi: "imsi-20893" + msin, Mcc: "208", Mnc: "93",
		Key: sub.Key, Opc: sub.Opc, Integrity: "nia2", Ciphering: "nea0", Dnn: "internet", Sst: 1, Sd: "010203"})
	require.NoError(t, err)
	return u
}

func TestRegisterThenEstablishPdu(t *testing.T) {
	assoc, amf, _ := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{} })
	u := newUE(t, "0000000001")

	link, out := Register(assoc, u, time.Second)
	require.Equal(t, metrics.Accepted, out.Result)
	require.NotNil(t, link)

	out = EstablishPdu(assoc, link, u, time.Second)
	require.Equal(t, metrics.Accepted, out.Result, out.Cause)
	require.Equal(t, netip.MustParseAddr("10.60.0.1"), out.UeIP)
	require.NotNil(t, out.Pdu)
	require.Equal(t, netip.MustParseAddr("10.0.1.5"), out.Pdu.UpfIP)
	require.Equal(t, []byte{0, 0, 0x10, 1}, out.Pdu.UlTeid)
	require.Equal(t, int64(9), out.Pdu.Qfi)

	require.Eventually(t, func() bool { return len(amf.SetupResponses()) == 1 }, time.Second, 5*time.Millisecond)
	rsp := amf.SetupResponses()[0]
	require.Equal(t, out.Pdu.DlTeid, rsp.DlTeid)
	require.Equal(t, netip.MustParseAddr("10.0.1.100"), rsp.GnbIP)
}

func TestRegistrationRejectedAndReleased(t *testing.T) {
	assoc, amf, _ := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{RejectRegistration: 7} })
	link, out := Register(assoc, newUE(t, "0000000001"), time.Second)
	require.Nil(t, link)
	require.Equal(t, Outcome{Result: metrics.Rejected, Cause: "5gmm(7)"}, out)
	require.Eventually(t, func() bool { return amf.ReleaseCompletes() == 1 }, time.Second, 5*time.Millisecond,
		"the gnb must answer the ue context release command")
}

func TestPduRejected(t *testing.T) {
	assoc, _, _ := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{RejectPdu: 27} })
	u := newUE(t, "0000000001")
	link, out := Register(assoc, u, time.Second)
	require.Equal(t, metrics.Accepted, out.Result)
	out = EstablishPdu(assoc, link, u, time.Second)
	require.Equal(t, Outcome{Result: metrics.Rejected, Cause: "5gsm(27)"}, out)
}

func TestPduTimeout(t *testing.T) {
	assoc, _, _ := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{IgnorePduRequest: true} })
	u := newUE(t, "0000000001")
	link, _ := Register(assoc, u, time.Second)
	start := time.Now()
	out := EstablishPdu(assoc, link, u, 100*time.Millisecond)
	require.Equal(t, Outcome{Result: metrics.TimedOut, Cause: "timeout"}, out)
	require.Less(t, time.Since(start), 500*time.Millisecond)
}

func TestAssociationLostMidRegistration(t *testing.T) {
	assoc, _, amfEnd := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{} })
	_ = amfEnd.Close() // the AMF goes away before anything is answered
	_, out := Register(assoc, newUE(t, "0000000001"), time.Second)
	require.Equal(t, metrics.Failed, out.Result)
	require.Contains(t, out.Cause, "association lost")
}

func TestManyUesInterleaveOnOneAssociation(t *testing.T) {
	assoc, amf, _ := setup(t, nil)
	const n = 50
	done := make(chan Outcome, n)
	for i := range n {
		go func() {
			u := newUE(t, "00000000"+string(rune('0'+i/10))+string(rune('0'+i%10)))
			link, out := Register(assoc, u, 2*time.Second)
			if out.Result != metrics.Accepted {
				done <- out
				return
			}
			done <- EstablishPdu(assoc, link, u, 2*time.Second)
		}()
	}
	ips := map[netip.Addr]bool{}
	for range n {
		out := <-done
		require.Equal(t, metrics.Accepted, out.Result, out.Cause)
		ips[out.UeIP] = true
	}
	require.Len(t, ips, n, "every UE gets its own address")
	require.Eventually(t, func() bool { return len(amf.SetupResponses()) == n }, time.Second, 5*time.Millisecond)
	teids := map[uint32]bool{}
	for _, r := range amf.SetupResponses() {
		teids[r.DlTeid] = true
	}
	require.Len(t, teids, n, "every UE gets its own DL TEID")
}

// Like a real UE, the tester waits for free5GC's Configuration Update
// Command after Registration Complete before asking for a PDU session.
func TestRegisterHandsOverOnlyAfterConfigurationUpdate(t *testing.T) {
	assoc, _, _ := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{} })
	u := newUE(t, "0000000001")
	link, out := Register(assoc, u, time.Second)
	require.Equal(t, metrics.Accepted, out.Result)
	require.False(t, out.DoneAt.IsZero(), "registration time ends at Registration Complete")
	select {
	case d := <-link.Downlinks:
		t.Fatalf("configuration update must be consumed by Register, got %+v", d)
	default:
	}
}

func TestRegisterDoesNotWaitForeverForConfigurationUpdate(t *testing.T) {
	old := configUpdateWait
	configUpdateWait = 50 * time.Millisecond
	t.Cleanup(func() { configUpdateWait = old })
	assoc, _, _ := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{NoConfigUpdate: true} })
	u := newUE(t, "0000000001")
	start := time.Now()
	link, out := Register(assoc, u, 5*time.Second)
	require.Equal(t, metrics.Accepted, out.Result)
	require.Less(t, time.Since(start), time.Second)
	require.Equal(t, metrics.Accepted, EstablishPdu(assoc, link, u, time.Second).Result)
}

// The AMF goes away while the UE is already waiting for its answer: the
// attempt must fail at once with "association lost", not sit out its
// timeout, even if the UE's downlink buffer could not take the event.
func TestAssociationLostWhileWaiting(t *testing.T) {
	gnbEnd, amfEnd := fakecore.Pipe()
	go func() {
		buf := make([]byte, 65536)
		_, _ = amfEnd.Read(buf) // take the Initial UE Message, then vanish
		_ = amfEnd.Close()
	}()
	id, err := gnb.NewIdentity(profile.GnbSpec{Name: "gNB-1", GnbID: "000314"},
		profile.GnbTemplate{Mcc: "208", Mnc: "93", Tac: "000001", Sst: 1, Sd: "010203"})
	require.NoError(t, err)
	assoc := gnb.NewAssociation(gnbEnd, id, netip.MustParseAddr("10.0.1.100"), &gnb.TeidAllocator{})
	go func() { _ = assoc.Run() }()

	start := time.Now()
	_, out := Register(assoc, newUE(t, "0000000001"), 5*time.Second)
	require.Equal(t, metrics.Failed, out.Result)
	require.Contains(t, out.Cause, "association lost")
	require.Less(t, time.Since(start), time.Second)
}

func TestAssociationLostIsNotDroppedWithAFullBuffer(t *testing.T) {
	gnbEnd, amfEnd := fakecore.Pipe()
	id, err := gnb.NewIdentity(profile.GnbSpec{Name: "gNB-1", GnbID: "000314"},
		profile.GnbTemplate{Mcc: "208", Mnc: "93", Tac: "000001", Sst: 1, Sd: "010203"})
	require.NoError(t, err)
	assoc := gnb.NewAssociation(gnbEnd, id, netip.MustParseAddr("10.0.1.100"), &gnb.TeidAllocator{})
	link, err := assoc.Attach()
	require.NoError(t, err)
	for range cap(link.Downlinks) { // a UE that stopped reading
		link.Downlinks <- gnb.Downlink{Kind: gnb.DownlinkNas}
	}
	go func() { _ = assoc.Run() }()
	_ = amfEnd.Close()

	select {
	case <-assoc.Lost():
	case <-time.After(time.Second):
		t.Fatal("Lost() must close when the association fails")
	}
	require.ErrorContains(t, assoc.Err(), "association lost")
}
