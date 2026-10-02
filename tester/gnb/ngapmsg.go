package gnb

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/free5gc/ngap/aper"
	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
)

// UE-associated NGAP messages the gNB sends, copied from free-ran-ue
// gnb/ngapBuilder.go and trimmed to what the tester uses.

func (id Identity) userLocation() *ie.UserLocationInformation {
	plmn := id.PlmnID
	return &ie.UserLocationInformation{
		Choice: &ie.UserLocationInformationNR{
			NRCGI: &ie.NRCGI{
				PLMNIdentity:   &plmn,
				NRCellIdentity: &ie.NRCellIdentity{Value: nrCellIdentity(id.GnbID)},
			},
			TAI: &ie.TAI{PLMNIdentity: id.Tai.PLMNIdentity, TAC: id.Tai.TAC},
		},
	}
}

// nrCellIdentity is the 36-bit NCI: the gNB ID in the high bits, cell 1.
func nrCellIdentity(gnbID []byte) aper.BitString {
	const nciBits = 36
	var v uint64
	for _, b := range gnbID {
		v = v<<8 | uint64(b)
	}
	idBits := len(gnbID) * 8
	if idBits > nciBits {
		v >>= uint(idBits - nciBits)
		idBits = nciBits
	}
	nci := v
	if cellBits := nciBits - idBits; cellBits > 0 {
		nci = v<<uint(cellBits) | 1
	}
	packed := nci << 4
	return aper.BitString{
		Bytes:     []byte{byte(packed >> 32), byte(packed >> 24), byte(packed >> 16), byte(packed >> 8), byte(packed)},
		BitLength: nciBits,
	}
}

func (id Identity) initialUEMessage(ranUeID int64, nas []byte) ([]byte, error) {
	return (&message.InitialUEMessage{
		RANUENGAPID:             &ie.RANUENGAPID{Value: ranUeID},
		NASPDU:                  &ie.NASPDU{Value: aper.OctetString(nas)},
		UserLocationInformation: id.userLocation(),
		RRCEstablishmentCause:   &ie.RRCEstablishmentCause{Value: ie.RRCEstablishmentCausePresentMoSignalling},
		UEContextRequest:        &ie.UEContextRequest{Value: ie.UEContextRequestPresentRequested},
	}).MarshalBinary()
}

func (id Identity) uplinkNASTransport(amfUeID, ranUeID int64, nas []byte) ([]byte, error) {
	return (&message.UplinkNASTransport{
		AMFUENGAPID:             &ie.AMFUENGAPID{Value: amfUeID},
		RANUENGAPID:             &ie.RANUENGAPID{Value: ranUeID},
		NASPDU:                  &ie.NASPDU{Value: aper.OctetString(nas)},
		UserLocationInformation: id.userLocation(),
	}).MarshalBinary()
}

func initialContextSetupResponse(amfUeID, ranUeID int64) ([]byte, error) {
	return (&message.InitialContextSetupResponse{
		AMFUENGAPID: &ie.AMFUENGAPID{Value: amfUeID},
		RANUENGAPID: &ie.RANUENGAPID{Value: ranUeID},
	}).MarshalBinary()
}

func (id Identity) ueContextReleaseComplete(amfUeID, ranUeID int64) ([]byte, error) {
	return (&message.UEContextReleaseComplete{
		AMFUENGAPID:             &ie.AMFUENGAPID{Value: amfUeID},
		RANUENGAPID:             &ie.RANUENGAPID{Value: ranUeID},
		UserLocationInformation: id.userLocation(),
	}).MarshalBinary()
}

func pduSessionResourceSetupResponse(amfUeID, ranUeID, pduSessionID int64, dlTeid uint32, gnbN3IP netip.Addr, qfi int64) ([]byte, error) {
	teid := make([]byte, 4)
	binary.BigEndian.PutUint32(teid, dlTeid)
	ip := gnbN3IP.As4()
	transfer := ie.PDUSessionResourceSetupResponseTransfer{
		DLQosFlowPerTNLInformation: &ie.QosFlowPerTNLInformation{
			UPTransportLayerInformation: &ie.UPTransportLayerInformation{
				Choice: &ie.GTPTunnel{
					GTPTEID:               &ie.GTPTEID{Value: aper.OctetString(teid)},
					TransportLayerAddress: &ie.TransportLayerAddress{Value: aper.BitString{Bytes: ip[:], BitLength: 32}},
				},
			},
			AssociatedQosFlowList: &ie.AssociatedQosFlowList{
				List: []ie.AssociatedQosFlowItem{{QosFlowIdentifier: &ie.QosFlowIdentifier{Value: qfi}}},
			},
		},
	}
	encoded, err := ie.MarshalBinary(&transfer)
	if err != nil {
		return nil, fmt.Errorf("encode pdu session resource setup response transfer: %w", err)
	}
	octets := aper.OctetString(encoded)
	return (&message.PDUSessionResourceSetupResponse{
		AMFUENGAPID: &ie.AMFUENGAPID{Value: amfUeID},
		RANUENGAPID: &ie.RANUENGAPID{Value: ranUeID},
		PDUSessionResourceSetupListSURes: &ie.PDUSessionResourceSetupListSURes{
			List: []ie.PDUSessionResourceSetupItemSURes{{
				PDUSessionID:                            &ie.PDUSessionID{Value: pduSessionID},
				PDUSessionResourceSetupResponseTransfer: &octets,
			}},
		},
	}).MarshalBinary()
}
