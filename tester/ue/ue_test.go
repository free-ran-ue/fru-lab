package ue

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"tester/internal/fakecore"
)

func testConfig() Config {
	return Config{
		Supi: "imsi-208930000000001", Mcc: "208", Mnc: "93",
		Key: "8baf473f2f8fd09487cccbd7097c6862", Opc: "8e27b6af0e692e750f32667a3b14605d",
		Integrity: "nia2", Ciphering: "nea0", Dnn: "internet", Sst: 1, Sd: "010203",
	}
}

func testSubscriber() fakecore.Subscriber {
	return fakecore.Subscriber{Mcc: "208", Mnc: "93",
		Key: "8baf473f2f8fd09487cccbd7097c6862", Opc: "8e27b6af0e692e750f32667a3b14605d",
		Amf: "8000", Sqn: "000000000023"}
}

// exchange feeds one uplink to the network and every downlink back to
// the UE, returning the UE's results in order.
func exchange(t *testing.T, u *UE, net *fakecore.NasSession, uplink []byte) []Result {
	t.Helper()
	dls, err := net.Handle(uplink)
	require.NoError(t, err)
	out := []Result{}
	for _, dl := range dls {
		r, err := u.Handle(dl.Nas)
		require.NoError(t, err)
		out = append(out, r)
	}
	return out
}

func register(t *testing.T, u *UE, net *fakecore.NasSession) Result {
	t.Helper()
	req, err := u.RegistrationRequest()
	require.NoError(t, err)
	uplink := req
	for range 5 {
		results := exchange(t, u, net, uplink)
		require.Len(t, results, 1)
		r := results[0]
		if r.Event != EventNone {
			return r
		}
		require.NotNil(t, r.Reply)
		uplink = r.Reply
	}
	t.Fatal("registration did not finish in 5 round trips")
	return Result{}
}

func TestRegistrationAndPduSession(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	net := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{}, netip.MustParseAddr("10.60.0.7"))

	r := register(t, u, net)
	require.Equal(t, EventRegistered, r.Event)
	require.NotNil(t, r.Reply, "registration complete must be sent")

	// Registration Complete triggers a Configuration Update Command: the
	// AMF's signal that it has finished processing the registration.
	results := exchange(t, u, net, r.Reply)
	require.Equal(t, []Result{{Event: EventConfigUpdate}}, results)

	req, err := u.PduSessionRequest()
	require.NoError(t, err)
	results = exchange(t, u, net, req)
	require.Len(t, results, 1)
	require.Equal(t, EventPduEstablished, results[0].Event)
	require.Equal(t, netip.MustParseAddr("10.60.0.7"), results[0].UeIP)
}

func TestRegistrationRejected(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	net := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{RejectRegistration: 7}, netip.Addr{})
	r := register(t, u, net)
	require.Equal(t, Result{Event: EventRegistrationRejected, Cause: "5gmm(7)"}, r)
}

func TestWrongKeyIsRejectedByNetwork(t *testing.T) {
	cfg := testConfig()
	cfg.Opc = "00000000000000000000000000000000"
	u, err := New(cfg)
	require.NoError(t, err)
	net := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{}, netip.Addr{})
	req, err := u.RegistrationRequest()
	require.NoError(t, err)
	dls, err := net.Handle(req)
	require.NoError(t, err)
	// With a wrong OPc the UE's AUTN MAC check fails locally.
	_, err = u.Handle(dls[0].Nas)
	require.ErrorContains(t, err, "verify autn")
}

func TestPduSessionRejected(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	net := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{RejectPdu: 27, NoConfigUpdate: true}, netip.Addr{})
	r := register(t, u, net)
	require.Empty(t, exchange(t, u, net, r.Reply))
	req, err := u.PduSessionRequest()
	require.NoError(t, err)
	results := exchange(t, u, net, req)
	require.Equal(t, []Result{{Event: EventPduRejected, Cause: "5gsm(27)"}}, results)
}

func TestPduBeforeRegistrationIsAnError(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	_, err = u.PduSessionRequest()
	require.ErrorContains(t, err, "before registration")
	_, err = u.Handle([]byte{0x7e, 0x00, 0x56})
	require.ErrorContains(t, err, "before registration request")
}

func TestRetryStartsFreshSecurityContext(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	first := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{}, netip.Addr{})
	require.Equal(t, EventRegistered, register(t, u, first).Event)
	second := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{}, netip.Addr{})
	require.Equal(t, EventRegistered, register(t, u, second).Event)
}

// establish registers u and sets up its PDU session against net.
func establish(t *testing.T, u *UE, net *fakecore.NasSession) {
	t.Helper()
	r := register(t, u, net)
	exchange(t, u, net, r.Reply)
	req, err := u.PduSessionRequest()
	require.NoError(t, err)
	require.Equal(t, EventPduEstablished, exchange(t, u, net, req)[0].Event)
}

func TestDeregistrationAfterPduSession(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	net := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{}, netip.MustParseAddr("10.60.0.7"))
	establish(t, u, net)

	dereg, err := u.DeregistrationRequest()
	require.NoError(t, err)
	require.Equal(t, []Result{{Event: EventDeregistered}}, exchange(t, u, net, dereg))
}

func TestDeregistrationWithoutPduSession(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	net := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{}, netip.MustParseAddr("10.60.0.7"))
	r := register(t, u, net)
	exchange(t, u, net, r.Reply)

	dereg, err := u.DeregistrationRequest()
	require.NoError(t, err)
	require.Equal(t, []Result{{Event: EventDeregistered}}, exchange(t, u, net, dereg))
}

func TestDeregistrationBeforeRegistrationIsAnError(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	_, err = u.DeregistrationRequest()
	require.Error(t, err)
}
