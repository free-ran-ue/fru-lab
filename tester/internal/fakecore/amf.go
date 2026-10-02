package fakecore

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/free5gc/ngap/aper"
	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
)

// AMF serves NGAP on gNB associations: NG Setup, then the UE-associated
// procedures, with NAS handled by one NasSession per UE.
type AMF struct {
	Subscriber Subscriber
	// Behavior picks the injected failures per SUPI; nil = all succeed.
	Behavior func(supi string) Behavior
	// UpfIP is put in every PDU Session Resource Setup Request.
	UpfIP netip.Addr

	nextAmfID atomic.Int64
	nextUeIP  atomic.Uint32
	mu        sync.Mutex
	setupRsps []SetupResponse
	released  int
	deregs    int
}

// SetupResponse is what a gNB answered to a PDU Session Resource Setup.
type SetupResponse struct {
	RanUeID int64
	DlTeid  uint32
	GnbIP   netip.Addr
}

func (a *AMF) SetupResponses() []SetupResponse {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]SetupResponse(nil), a.setupRsps...)
}

// Deregistrations counts Deregistration Accepts sent.
func (a *AMF) Deregistrations() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.deregs
}

func (a *AMF) ReleaseCompletes() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.released
}

type amfUe struct {
	amfID, ranID int64
	nas          *NasSession
}

// Serve handles one association until the conn is closed.
func (a *AMF) Serve(conn io.ReadWriter) error {
	ues := map[int64]*amfUe{} // by AMF UE NGAP ID
	buf := make([]byte, 65536)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return err
		}
		msg, err := message.Parse(buf[:n])
		if err != nil {
			return fmt.Errorf("fake amf: decode: %w", err)
		}
		switch m := msg.(type) {
		case *message.NGSetupRequest:
			raw, encErr := ngSetupResponse(m)
			err = a.write(conn, raw, encErr)
		case *message.InitialUEMessage:
			ue := &amfUe{amfID: a.nextAmfID.Add(1), ranID: m.RANUENGAPID.Value}
			ue.nas = NewNasSession(a.Subscriber, Behavior{}, a.allocateUeIP())
			ues[ue.amfID] = ue
			err = a.onNas(conn, ue, m.NASPDU.Value)
		case *message.UplinkNASTransport:
			ue := ues[m.AMFUENGAPID.Value]
			if ue == nil {
				return fmt.Errorf("fake amf: unknown amf ue id %d", m.AMFUENGAPID.Value)
			}
			err = a.onNas(conn, ue, m.NASPDU.Value)
		case *message.PDUSessionResourceSetupResponse:
			a.recordSetupResponse(m)
		case *message.UEContextReleaseComplete:
			a.mu.Lock()
			a.released++
			a.mu.Unlock()
		}
		if err != nil {
			return err
		}
	}
}

func (a *AMF) allocateUeIP() netip.Addr {
	n := a.nextUeIP.Add(1)
	return netip.AddrFrom4([4]byte{10, 60, byte(n >> 8), byte(n)})
}

func (a *AMF) onNas(conn io.Writer, ue *amfUe, nas []byte) error {
	if ue.nas.state == "idle" && a.Behavior != nil {
		// pick the behavior once the SUPI is known (from the first message)
		probe := NewNasSession(a.Subscriber, Behavior{}, netip.Addr{})
		_, _ = probe.onRegistrationRequest(nas)
		ue.nas.behavior = a.Behavior(probe.supi)
	}
	dls, err := ue.nas.Handle(nas)
	if err != nil {
		return fmt.Errorf("fake amf: nas: %w", err)
	}
	for _, dl := range dls {
		var raw []byte
		switch {
		case dl.InitialSetup:
			raw, err = initialContextSetupRequest(ue, dl.Nas)
		case dl.PduSetup:
			raw, err = pduSessionResourceSetupRequest(ue, dl.Nas, a.UpfIP)
		default:
			raw, err = (&message.DownlinkNASTransport{
				AMFUENGAPID: &ie.AMFUENGAPID{Value: ue.amfID},
				RANUENGAPID: &ie.RANUENGAPID{Value: ue.ranID},
				NASPDU:      &ie.NASPDU{Value: aper.OctetString(dl.Nas)},
			}).MarshalBinary()
		}
		if err == nil {
			err = a.write(conn, raw, nil)
		}
		if err == nil && dl.Release {
			a.mu.Lock()
			a.deregs++
			a.mu.Unlock()
			raw, err = ueContextReleaseCommand(ue)
			if err == nil {
				err = a.write(conn, raw, nil)
			}
		}
		if err != nil {
			return err
		}
	}
	if ue.nas.state == "done" && ue.nas.behavior.RejectRegistration != 0 {
		raw, err := ueContextReleaseCommand(ue)
		if err == nil {
			err = a.write(conn, raw, nil)
		}
		return err
	}
	return nil
}

func ueContextReleaseCommand(ue *amfUe) ([]byte, error) {
	return (&message.UEContextReleaseCommand{
		UENGAPIDs: &ie.UENGAPIDs{Choice: &ie.UENGAPIDPair{
			AMFUENGAPID: &ie.AMFUENGAPID{Value: ue.amfID},
			RANUENGAPID: &ie.RANUENGAPID{Value: ue.ranID},
		}},
		Cause: &ie.Cause{Choice: &ie.CauseNas{Value: ie.CauseNasPresentNormalRelease}},
	}).MarshalBinary()
}

func (a *AMF) write(conn io.Writer, raw []byte, err error) error {
	if err != nil {
		return err
	}
	_, err = conn.Write(raw)
	return err
}

func (a *AMF) recordSetupResponse(m *message.PDUSessionResourceSetupResponse) {
	if m.PDUSessionResourceSetupListSURes == nil || len(m.PDUSessionResourceSetupListSURes.List) == 0 {
		return
	}
	item := m.PDUSessionResourceSetupListSURes.List[0]
	var t ie.PDUSessionResourceSetupResponseTransfer
	if item.PDUSessionResourceSetupResponseTransfer == nil ||
		ie.UnmarshalBinary(*item.PDUSessionResourceSetupResponseTransfer, &t) != nil ||
		t.DLQosFlowPerTNLInformation == nil {
		return
	}
	tun, ok := t.DLQosFlowPerTNLInformation.UPTransportLayerInformation.Choice.(*ie.GTPTunnel)
	if !ok {
		return
	}
	ip, _ := netip.AddrFromSlice(tun.TransportLayerAddress.Value.Bytes)
	a.mu.Lock()
	a.setupRsps = append(a.setupRsps, SetupResponse{
		RanUeID: m.RANUENGAPID.Value, DlTeid: binary.BigEndian.Uint32(tun.GTPTEID.Value), GnbIP: ip,
	})
	a.mu.Unlock()
}

func ngSetupResponse(req *message.NGSetupRequest) ([]byte, error) {
	plmn := *req.GlobalRANNodeID.Choice.(*ie.GlobalGNBID).PLMNIdentity
	snssai := ie.SNSSAI{SST: &ie.SST{Value: aper.OctetString{1}}}
	return (&message.NGSetupResponse{
		AMFName: &ie.AMFName{Value: "fake-amf"},
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
}

func initialContextSetupRequest(ue *amfUe, nas []byte) ([]byte, error) {
	plmn := ie.PLMNIdentity{Value: aper.OctetString{0x02, 0xf8, 0x39}}
	return (&message.InitialContextSetupRequest{
		AMFUENGAPID: &ie.AMFUENGAPID{Value: ue.amfID},
		RANUENGAPID: &ie.RANUENGAPID{Value: ue.ranID},
		GUAMI: &ie.GUAMI{
			PLMNIdentity: &plmn,
			AMFRegionID:  &ie.AMFRegionID{Value: aper.BitString{Bytes: []byte{0xca}, BitLength: 8}},
			AMFSetID:     &ie.AMFSetID{Value: aper.BitString{Bytes: []byte{0xfe, 0x00}, BitLength: 10}},
			AMFPointer:   &ie.AMFPointer{Value: aper.BitString{Bytes: []byte{0x00}, BitLength: 6}},
		},
		AllowedNSSAI: &ie.AllowedNSSAI{List: []ie.AllowedNSSAIItem{{SNSSAI: &ie.SNSSAI{SST: &ie.SST{Value: aper.OctetString{1}}}}}},
		UESecurityCapabilities: &ie.UESecurityCapabilities{
			NRencryptionAlgorithms:             &ie.NRencryptionAlgorithms{Value: aper.BitString{Bytes: []byte{0, 0}, BitLength: 16}},
			NRintegrityProtectionAlgorithms:    &ie.NRintegrityProtectionAlgorithms{Value: aper.BitString{Bytes: []byte{0, 0}, BitLength: 16}},
			EUTRAencryptionAlgorithms:          &ie.EUTRAencryptionAlgorithms{Value: aper.BitString{Bytes: []byte{0, 0}, BitLength: 16}},
			EUTRAintegrityProtectionAlgorithms: &ie.EUTRAintegrityProtectionAlgorithms{Value: aper.BitString{Bytes: []byte{0, 0}, BitLength: 16}},
		},
		SecurityKey: &ie.SecurityKey{Value: aper.BitString{Bytes: make([]byte, 32), BitLength: 256}},
		NASPDU:      &ie.NASPDU{Value: aper.OctetString(nas)},
	}).MarshalBinary()
}

func pduSessionResourceSetupRequest(ue *amfUe, nas []byte, upfIP netip.Addr) ([]byte, error) {
	ip := upfIP.As4()
	transfer := ie.PDUSessionResourceSetupRequestTransfer{
		ProtocolIEs: &ie.ProtocolIEContainerPDUSessionResourceSetupRequestTransferIEs{
			List: []ie.PDUSessionResourceSetupRequestTransferIEs{
				{ULNGUUPTNLInformation: &ie.UPTransportLayerInformation{Choice: &ie.GTPTunnel{
					TransportLayerAddress: &ie.TransportLayerAddress{Value: aper.BitString{Bytes: ip[:], BitLength: 32}},
					GTPTEID:               &ie.GTPTEID{Value: aper.OctetString{0, 0, 0x10, byte(ue.amfID)}},
				}}},
				{PDUSessionType: &ie.PDUSessionType{Value: ie.PDUSessionTypePresentIpv4}},
				{QosFlowSetupRequestList: &ie.QosFlowSetupRequestList{List: []ie.QosFlowSetupRequestItem{{
					QosFlowIdentifier: &ie.QosFlowIdentifier{Value: 9},
					QosFlowLevelQosParameters: &ie.QosFlowLevelQosParameters{
						QosCharacteristics: &ie.QosCharacteristics{Choice: &ie.NonDynamic5QIDescriptor{FiveQI: &ie.FiveQI{Value: 9}}},
						AllocationAndRetentionPriority: &ie.AllocationAndRetentionPriority{
							PriorityLevelARP:        &ie.PriorityLevelARP{Value: 8},
							PreEmptionCapability:    &ie.PreEmptionCapability{Value: ie.PreEmptionCapabilityPresentShallNotTriggerPreEmption},
							PreEmptionVulnerability: &ie.PreEmptionVulnerability{Value: ie.PreEmptionVulnerabilityPresentNotPreEmptable},
						},
					},
				}}}},
			},
		},
	}
	encoded, err := ie.MarshalBinary(&transfer)
	if err != nil {
		return nil, fmt.Errorf("fake amf: encode setup request transfer: %w", err)
	}
	octets := aper.OctetString(encoded)
	return (&message.PDUSessionResourceSetupRequest{
		AMFUENGAPID: &ie.AMFUENGAPID{Value: ue.amfID},
		RANUENGAPID: &ie.RANUENGAPID{Value: ue.ranID},
		PDUSessionResourceSetupListSUReq: &ie.PDUSessionResourceSetupListSUReq{List: []ie.PDUSessionResourceSetupItemSUReq{{
			PDUSessionID:                           &ie.PDUSessionID{Value: 1},
			PDUSessionNASPDU:                       &ie.NASPDU{Value: aper.OctetString(nas)},
			SNSSAI:                                 &ie.SNSSAI{SST: &ie.SST{Value: aper.OctetString{1}}},
			PDUSessionResourceSetupRequestTransfer: &octets,
		}}},
	}).MarshalBinary()
}
