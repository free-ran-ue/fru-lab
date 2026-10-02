package gnb

import (
	"bytes"
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/free5gc/ngap/aper"
	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
	"github.com/stretchr/testify/require"

	"tester/profile"
)

func testIdentity(t *testing.T) Identity {
	t.Helper()
	id, err := NewIdentity(
		profile.GnbSpec{Index: 1, Name: "gNB-1", GnbID: "000314"},
		profile.GnbTemplate{Mcc: "208", Mnc: "93", Tac: "000001", Sst: 1, Sd: "010203"},
	)
	require.NoError(t, err)
	return id
}

func ngSetupResponse(t *testing.T, id Identity) []byte {
	t.Helper()
	plmn, snssai := id.PlmnID, id.Snssai
	b, err := (&message.NGSetupResponse{
		AMFName: &ie.AMFName{Value: "AMF"},
		ServedGUAMIList: &ie.ServedGUAMIList{List: []ie.ServedGUAMIItem{{GUAMI: &ie.GUAMI{
			PLMNIdentity: &plmn,
			AMFRegionID:  &ie.AMFRegionID{Value: aper.BitString{Bytes: []byte{0xca}, BitLength: 8}},
			AMFSetID:     &ie.AMFSetID{Value: aper.BitString{Bytes: []byte{0xfe, 0x00}, BitLength: 10}},
			AMFPointer:   &ie.AMFPointer{Value: aper.BitString{Bytes: []byte{0x00}, BitLength: 6}},
		}}}},
		RelativeAMFCapacity: &ie.RelativeAMFCapacity{Value: 255},
		PLMNSupportList: &ie.PLMNSupportList{List: []ie.PLMNSupportItem{{
			PLMNIdentity:     &plmn,
			SliceSupportList: &ie.SliceSupportList{List: []ie.SliceSupportItem{{SNSSAI: &snssai}}},
		}}},
	}).MarshalBinary()
	require.NoError(t, err)
	return b
}

// fakeAMF answers one NG Setup Request on its end of a net.Pipe.
func fakeAMF(t *testing.T, reply []byte) net.Conn {
	t.Helper()
	gnbEnd, amfEnd := net.Pipe()
	go func() {
		defer func() { _ = amfEnd.Close() }()
		buf := make([]byte, 4096)
		n, err := amfEnd.Read(buf)
		if err != nil {
			return
		}
		if _, err := message.Parse(buf[:n]); err != nil {
			return
		}
		_, _ = amfEnd.Write(reply)
	}()
	t.Cleanup(func() { _ = gnbEnd.Close() })
	return gnbEnd
}

func TestNGSetupRequestRoundTrips(t *testing.T) {
	id := testIdentity(t)
	raw, err := id.NGSetupRequest()
	require.NoError(t, err)
	msg, err := message.Parse(raw)
	require.NoError(t, err)
	req, ok := msg.(*message.NGSetupRequest)
	require.True(t, ok)
	require.Equal(t, "gNB-1", string(req.RANNodeName.Value))
	gnbID := req.GlobalRANNodeID.Choice.(*ie.GlobalGNBID).GNBID.Choice.(*ie.GNBIDForGNBID).Value
	require.Equal(t, []byte{0x00, 0x03, 0x14}, gnbID.Bytes)
	require.Equal(t, uint64(24), gnbID.BitLength)
}

func TestExchangeNGSetupAccepted(t *testing.T) {
	id := testIdentity(t)
	req, err := id.NGSetupRequest()
	require.NoError(t, err)
	require.NoError(t, ExchangeNGSetup(fakeAMF(t, ngSetupResponse(t, id)), req))
}

func TestExchangeNGSetupRejectedCarriesCause(t *testing.T) {
	id := testIdentity(t)
	req, err := id.NGSetupRequest()
	require.NoError(t, err)
	failure, err := (&message.NGSetupFailure{
		Cause: &ie.Cause{Choice: &ie.CauseMisc{Value: ie.CauseMiscPresentUnknownPLMNOrSNPN}},
	}).MarshalBinary()
	require.NoError(t, err)

	err = ExchangeNGSetup(fakeAMF(t, failure), req)
	var rejected *RejectedError
	require.ErrorAs(t, err, &rejected)
	require.Equal(t, "misc(4)", rejected.Cause)
	require.EqualError(t, err, "ng setup rejected: misc(4)")
}

func TestExchangeNGSetupGarbageReply(t *testing.T) {
	id := testIdentity(t)
	req, err := id.NGSetupRequest()
	require.NoError(t, err)
	err = ExchangeNGSetup(fakeAMF(t, []byte{0xde, 0xad}), req)
	require.ErrorContains(t, err, "decode ng setup response")
	var rejected *RejectedError
	require.False(t, errors.As(err, &rejected))
}

// timeoutConn behaves like an SCTP socket whose SO_RCVTIMEO expired.
type timeoutConn struct{ bytes.Buffer }

func (c *timeoutConn) Read([]byte) (int, error) { return 0, syscall.EAGAIN }

func TestExchangeNGSetupReadTimeoutKeepsErrno(t *testing.T) {
	id := testIdentity(t)
	req, err := id.NGSetupRequest()
	require.NoError(t, err)
	err = ExchangeNGSetup(&timeoutConn{}, req)
	require.ErrorIs(t, err, syscall.EAGAIN)
}

func TestDescribeCause(t *testing.T) {
	require.Equal(t, "unknown", DescribeCause(nil))
	require.Equal(t, "radioNetwork(0)", DescribeCause(&ie.Cause{Choice: &ie.CauseRadioNetwork{Value: 0}}))
}
