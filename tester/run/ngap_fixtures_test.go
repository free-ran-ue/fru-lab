package run

import (
	"testing"

	"github.com/free5gc/ngap/aper"
	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
	"github.com/stretchr/testify/require"
)

func ngSetupResponseBytes(t *testing.T) []byte {
	t.Helper()
	plmn := ie.PLMNIdentity{Value: aper.OctetString{0x02, 0xf8, 0x39}}
	snssai := ie.SNSSAI{SST: &ie.SST{Value: aper.OctetString{1}}}
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

func ngSetupFailureBytes(t *testing.T) []byte {
	t.Helper()
	b, err := (&message.NGSetupFailure{
		Cause: &ie.Cause{Choice: &ie.CauseMisc{Value: ie.CauseMiscPresentUnknownPLMNOrSNPN}},
	}).MarshalBinary()
	require.NoError(t, err)
	return b
}
