// Package fakecore is a minimal network side for tests: the NAS half of an
// AMF+SMF (5G-AKA, security mode, registration, PDU session) and, in
// amf.go, an NGAP AMF that serves gNB associations. It implements the key
// derivations independently of package ue, so tests cross-check them.
package fakecore

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"

	"github.com/free5gc/nas/ie"
	"github.com/free5gc/nas/message"
	"github.com/free5gc/util/milenage"
	"github.com/free5gc/util/ueauth"
)

// Subscriber is what the fake UDM knows about one UE.
type Subscriber struct {
	Mcc, Mnc string
	Key, Opc string // 32 hex
	Amf      string // 4 hex
	Sqn      string // 12 hex
}

// Behavior injects failures. Zero value = everything succeeds.
type Behavior struct {
	RejectRegistration uint8 // 5GMM cause; 0 = accept
	RejectPdu          uint8 // 5GSM cause; 0 = accept
	NoConfigUpdate     bool  // skip the Configuration Update Command free5GC sends after Registration Complete
	IgnorePduRequest   bool  // never answer the PDU request (for timeouts)
	BadAutn            bool  // corrupt AUTN so the UE's MAC check fails
	// IgnoreDeregistration never answers the deregistration request (for timeouts).
	IgnoreDeregistration bool
}

// NasSession is the network side of one UE's NAS signalling.
type NasSession struct {
	sub      Subscriber
	behavior Behavior
	ueIP     netip.Addr

	supi     string
	xresStar []byte
	kAmf     []byte
	secCtx   *message.SecCtx
	state    string
}

func NewNasSession(sub Subscriber, b Behavior, ueIP netip.Addr) *NasSession {
	return &NasSession{sub: sub, behavior: b, ueIP: ueIP, state: "idle"}
}

// Downlink is one NAS PDU for the UE. Nas is the PDU; PduSetup marks the
// PDU Session Establishment Accept, which the AMF must carry inside a PDU
// Session Resource Setup Request instead of a Downlink NAS Transport.
type Downlink struct {
	Nas          []byte
	InitialSetup bool // carry in Initial Context Setup Request (Registration Accept)
	PduSetup     bool
	Release      bool // follow with a UE Context Release Command (after Deregistration Accept)
}

// Handle consumes one uplink NAS PDU and returns the downlink replies.
func (s *NasSession) Handle(uplink []byte) ([]Downlink, error) {
	switch s.state {
	case "idle":
		return s.onRegistrationRequest(uplink)
	case "auth":
		return s.onAuthResponse(uplink)
	case "smc":
		return s.onSecurityModeComplete(uplink)
	case "accepted":
		return s.onRegistrationComplete(uplink)
	case "registered", "established":
		return s.onRegisteredUplink(uplink)
	default:
		return nil, fmt.Errorf("unexpected uplink in state %s", s.state)
	}
}

func (s *NasSession) onRegistrationRequest(b []byte) ([]Downlink, error) {
	m, err := message.ParseGMM(b)
	if err != nil {
		return nil, fmt.Errorf("decode registration request: %w", err)
	}
	req, ok := m.(*message.RegReq)
	if !ok || req.MobileId5GS == nil {
		return nil, fmt.Errorf("expected registration request, got %T", m)
	}
	s.supi = "imsi-" + supiDigits(req.MobileId5GS)
	if s.behavior.RejectRegistration != 0 {
		s.state = "done"
		rej, err := (&message.RegRej{Cause5GMM: &ie.Cause5GMM{Value: s.behavior.RejectRegistration}}).MarshalBinary()
		return []Downlink{{Nas: rej}}, err
	}

	k, _ := hex.DecodeString(s.sub.Key)
	opc, _ := hex.DecodeString(s.sub.Opc)
	sqn, _ := hex.DecodeString(s.sub.Sqn)
	amf, _ := hex.DecodeString(s.sub.Amf)
	r := make([]byte, 16)
	_, _ = rand.Read(r)
	ik, ck, xres, autn, err := milenage.GenerateAKAParameters(opc, k, r, sqn, amf)
	if err != nil {
		return nil, err
	}
	if s.behavior.BadAutn {
		autn[len(autn)-1] ^= 0xff
	}
	sn := snName(s.sub.Mcc, s.sub.Mnc)
	ckik := append(append([]byte{}, ck...), ik...)
	out, err := ueauth.GetKDFValue(ckik, ueauth.FC_FOR_RES_STAR_XRES_STAR_DERIVATION,
		[]byte(sn), ueauth.KDFLen([]byte(sn)), r, ueauth.KDFLen(r), xres, ueauth.KDFLen(xres))
	if err != nil {
		return nil, err
	}
	s.xresStar = out[len(out)/2:]
	// SQN xor AK is the first 6 bytes of AUTN.
	kausf, _ := ueauth.GetKDFValue(ckik, ueauth.FC_FOR_KAUSF_DERIVATION,
		[]byte(sn), ueauth.KDFLen([]byte(sn)), autn[:6], ueauth.KDFLen(autn[:6]))
	kseaf, _ := ueauth.GetKDFValue(kausf, ueauth.FC_FOR_KSEAF_DERIVATION, []byte(sn), ueauth.KDFLen([]byte(sn)))
	p0, p1 := []byte(s.supi[len("imsi-"):]), []byte{0, 0}
	s.kAmf, _ = ueauth.GetKDFValue(kseaf, ueauth.FC_FOR_KAMF_DERIVATION, p0, ueauth.KDFLen(p0), p1, ueauth.KDFLen(p1))

	s.state = "auth"
	req2, err := (&message.AuthReq{
		Ngksi:                   &ie.NASKeySetId{Tsc: ie.SecCtxTypeNative, Ksi: 0},
		ABBA:                    &ie.ABBA{Abba: []byte{0, 0}},
		AuthParamRAND5GAuthChlg: &ie.AuthParamRAND{Rand: r},
		AuthParamAUTN5GAuthChlg: &ie.AuthParamAUTN{Autn: autn},
	}).MarshalBinary()
	return []Downlink{{Nas: req2}}, err
}

func (s *NasSession) onAuthResponse(b []byte) ([]Downlink, error) {
	m, err := message.ParseGMM(b)
	if err != nil {
		return nil, fmt.Errorf("decode authentication response: %w", err)
	}
	rsp, ok := m.(*message.AuthRsp)
	if !ok || rsp.AuthRspParam == nil {
		if _, failed := m.(*message.AuthFailure); failed {
			s.state = "done"
			return nil, errors.New("ue sent authentication failure")
		}
		return nil, fmt.Errorf("expected authentication response, got %T", m)
	}
	if !bytes.Equal(rsp.AuthRspParam.Res, s.xresStar) {
		s.state = "done"
		rej, err := (&message.AuthRej{}).MarshalBinary()
		return []Downlink{{Nas: rej}}, err
	}
	kenc, kint := nasKeys(s.kAmf, message.AlgCiphering128NEA0, message.AlgIntegrity128NIA2)
	s.secCtx = &message.SecCtx{
		Side: message.CoreNetworkSide, Bearer: message.Bearer3GPP,
		UplinkCount: &message.Count{}, DownlinkCount: &message.Count{},
		CipheringAlg: message.AlgCiphering128NEA0, IntegrityAlg: message.AlgIntegrity128NIA2,
		KnasEnc: kenc, KnasInt: kint,
	}
	s.state = "smc"
	smc, err := message.Marshal(&message.SecModeCmd{
		SelectedNASSecAlgos:       &ie.NASSecAlgos{CipheringAlgo: message.AlgCiphering128NEA0, MsgIntAlgo: message.AlgIntegrity128NIA2},
		Ngksi:                     &ie.NASKeySetId{Tsc: ie.SecCtxTypeNative, Ksi: 0},
		ReplayedUESecCapabilities: &ie.UESecCapability{Length: 2, EA05G: true, IA2_128_5G: true},
	}, s.secCtx, message.SecHdrTypeIntegrityProtectedWithNew5gNasSecCtx)
	return []Downlink{{Nas: smc}}, err
}

func (s *NasSession) onSecurityModeComplete(b []byte) ([]Downlink, error) {
	m, err := message.Parse(b, s.secCtx)
	if err != nil {
		return nil, fmt.Errorf("decode security mode complete: %w", err)
	}
	if _, ok := m.(*message.SecModeComplete); !ok {
		return nil, fmt.Errorf("expected security mode complete, got %T", m)
	}
	s.state = "accepted"
	acc, err := message.Marshal(&message.RegAccept{
		RegResult5GS: &ie.RegResult5GS{Value: ie.RegResult_3gpp},
	}, s.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
	return []Downlink{{Nas: acc, InitialSetup: true}}, err
}

func (s *NasSession) onRegistrationComplete(b []byte) ([]Downlink, error) {
	m, err := message.Parse(b, s.secCtx)
	if err != nil {
		return nil, fmt.Errorf("decode registration complete: %w", err)
	}
	if _, ok := m.(*message.RegComplete); !ok {
		return nil, fmt.Errorf("expected registration complete, got %T", m)
	}
	s.state = "registered"
	if s.behavior.NoConfigUpdate {
		return nil, nil
	}
	cmd, err := message.Marshal(&message.CfgUpdateCmd{}, s.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
	return []Downlink{{Nas: cmd}}, err
}

// onRegisteredUplink handles what a registered UE sends: a PDU session
// request (once) or a deregistration request.
func (s *NasSession) onRegisteredUplink(b []byte) ([]Downlink, error) {
	m, err := message.Parse(b, s.secCtx)
	if err != nil {
		return nil, fmt.Errorf("decode uplink: %w", err)
	}
	if _, ok := m.(*message.DeregReqUEOrig); ok {
		return s.onDeregistration()
	}
	if s.state == "established" {
		return nil, fmt.Errorf("unexpected %T after the pdu session", m)
	}
	return s.onPduRequest(m)
}

func (s *NasSession) onDeregistration() ([]Downlink, error) {
	s.state = "deregistered"
	if s.behavior.IgnoreDeregistration {
		return nil, nil
	}
	acc, err := message.Marshal(&message.DeregAcceptUEOrig{}, s.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
	return []Downlink{{Nas: acc, Release: true}}, err
}

func (s *NasSession) onPduRequest(m message.Message) ([]Downlink, error) {
	ul, ok := m.(*message.ULNASTransport)
	if !ok || ul.PayloadCntr == nil {
		return nil, fmt.Errorf("expected ul nas transport, got %T", m)
	}
	if _, err := message.ParseGSM(ul.PayloadCntr.Contents); err != nil {
		return nil, fmt.Errorf("decode pdu session establishment request: %w", err)
	}
	if s.behavior.IgnorePduRequest {
		return nil, nil
	}
	// a rejected PDU request leaves the UE registered, as in a real core
	var gsm []byte
	var err error
	pduSetup := false
	if s.behavior.RejectPdu != 0 {
		gsm, err = (&message.PDUSessEstRej{PDUSessId: 1, Cause5GSM: &ie.Cause5GSM{Value: s.behavior.RejectPdu}}).MarshalBinary()
	} else {
		ip := s.ueIP.As4()
		gsm, err = (&message.PDUSessEstAccept{
			PDUSessId:           1,
			SelectedPDUSessType: &ie.PDUSessType{Value: ie.PDUSessType_IPv4},
			SelectedSSCMode:     &ie.SSCMode{Mode: ie.SSCMODE1},
			AuthoQosRules: &ie.QosRules{Rules: []ie.QosRule{{ // default rule, any-to-any, QFI 1
				RuleId: 1, OpCode: ie.OpCode_CreateNewQosRule, IsDefaultDQR: true, Precedence: 255, QFI: 1,
				PktFilterList: []ie.PacketFilter{{Dir: 3, Id: 1, Contents: ie.PacketFilterContents{RemoteAddr: "any", LocalAddr: "any"}}},
			}}},
			SessAMBR: &ie.SessAMBR{UnitDownlink: ie.Rate_1Mbps, ValueDownlink: 100, UnitUplink: ie.Rate_1Mbps, ValueUplink: 100},
			PDUAddr:  &ie.PDUAddr{IPv4: ip[:]},
		}).MarshalBinary()
		pduSetup = true
		s.state = "established"
	}
	if err != nil {
		return nil, fmt.Errorf("encode 5gsm reply: %w", err)
	}
	dl, err := message.Marshal(&message.DLNASTransport{
		PayloadCntrType: &ie.PayloadCntrType{Value: ie.PayloadCntrType_N1SMInfo},
		PayloadCntr:     &ie.PayloadCntr{Pct: ie.PayloadCntrType_N1SMInfo, Contents: gsm},
		PDUSessID:       &ie.PDUSessId2{Value: 1},
	}, s.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
	return []Downlink{{Nas: dl, PduSetup: pduSetup}}, err
}

func nasKeys(kAmf []byte, enc ie.AlgCiphering, integ ie.AlgIntegrity) (kenc, kint [16]byte) {
	p0, p1 := []byte{message.NNASEncAlg}, []byte{byte(enc)}
	e, _ := ueauth.GetKDFValue(kAmf, ueauth.FC_FOR_ALGORITHM_KEY_DERIVATION, p0, ueauth.KDFLen(p0), p1, ueauth.KDFLen(p1))
	p0, p1 = []byte{message.NNASIntAlg}, []byte{byte(integ)}
	i, _ := ueauth.GetKDFValue(kAmf, ueauth.FC_FOR_ALGORITHM_KEY_DERIVATION, p0, ueauth.KDFLen(p0), p1, ueauth.KDFLen(p1))
	copy(kenc[:], e[16:])
	copy(kint[:], i[16:])
	return kenc, kint
}

func snName(mcc, mnc string) string {
	if len(mnc) == 2 {
		mnc = "0" + mnc
	}
	return "5G:mnc" + mnc + ".mcc" + mcc + ".3gppnetwork.org"
}

// supiDigits decodes the SUCI (null scheme) MCC+MNC+MSIN digits.
func supiDigits(id *ie.MobileId5GS) string {
	raw, _ := id.MarshalBinary()
	// raw: type octet, PLMN (3 octets BCD), routing indicator (2), scheme, hnpki, msin BCD
	bcd := func(b byte) string {
		lo, hi := b&0x0f, b>>4
		s := fmt.Sprintf("%d", lo)
		if hi != 0x0f {
			s += fmt.Sprintf("%d", hi)
		}
		return s
	}
	plmn := raw[1:4]
	mcc := fmt.Sprintf("%d%d%d", plmn[0]&0x0f, plmn[0]>>4, plmn[1]&0x0f)
	mnc := fmt.Sprintf("%d%d", plmn[2]&0x0f, plmn[2]>>4)
	if plmn[1]>>4 != 0x0f {
		mnc += fmt.Sprintf("%d", plmn[1]>>4)
	}
	msin := ""
	for _, b := range raw[8:] {
		msin += bcd(b)
	}
	return mcc + mnc + msin
}
