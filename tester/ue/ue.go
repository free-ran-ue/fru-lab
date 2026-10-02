// Package ue is one simulated UE's NAS layer: it builds uplink NAS
// messages and reacts to downlink ones, but does no I/O. The gNB layer
// carries the bytes; the procedure layer decides what to send when.
package ue

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"

	"github.com/free-ran-ue/util"
	"github.com/free5gc/nas/ie"
	"github.com/free5gc/nas/message"

	"tester/profile"
)

// PduSessionID is the one PDU session each UE establishes (spec Q14).
const PduSessionID uint8 = 1

// Config is everything a UE needs, already validated by profile.Expand.
type Config struct {
	Supi      string // "imsi-" + digits
	Mcc, Mnc  string
	Key, Opc  string // 32 hex
	Integrity string // nia0..nia3
	Ciphering string // nea0..nea3
	Dnn       string
	Sst       int
	Sd        string
}

// ConfigFrom combines one UeSpec with the templates it was expanded from.
func ConfigFrom(spec profile.UeSpec, g profile.GnbTemplate, u profile.UeTemplate) Config {
	return Config{
		Supi: spec.Supi, Mcc: g.Mcc, Mnc: g.Mnc, Key: u.Key, Opc: u.Opc,
		Integrity: u.Integrity, Ciphering: u.Ciphering, Dnn: u.Dnn, Sst: u.Sst, Sd: u.Sd,
	}
}

// Event says what a downlink message meant for the procedure in progress.
type Event int

const (
	EventNone                 Event = iota // keep waiting (reply, if any, still has to be sent)
	EventRegistered                        // Registration Accept handled; Reply is Registration Complete
	EventRegistrationRejected              // Registration/Authentication Reject; Cause set
	EventPduEstablished                    // PDU Session Establishment Accept; UeIP set
	EventPduRejected                       // PDU Session Establishment Reject or 5GMM cause; Cause set
	EventConfigUpdate                      // Configuration Update Command: the AMF finished the registration
)

// Result is the outcome of handling one downlink NAS message.
type Result struct {
	Reply []byte // uplink NAS to send back, nil if none
	Event Event
	Cause string // e.g. "5gmm(7)", "5gsm(27)", "authentication reject"
	UeIP  netip.Addr
}

// UE holds one UE's NAS security context. Not safe for concurrent use:
// a UE is driven by one procedure goroutine at a time.
type UE struct {
	cfg      Config
	digits   string // MCC+MNC+MSIN
	k, opc   []byte
	integ    ie.AlgIntegrity
	enc      ie.AlgCiphering
	secCap   *ie.UESecCapability
	mobileID *ie.MobileId5GS
	secCtx   *message.SecCtx
	kAmf     []byte
}

func New(cfg Config) (*UE, error) {
	k, err := hex.DecodeString(cfg.Key)
	if err != nil {
		return nil, fmt.Errorf("key: %w", err)
	}
	opc, err := hex.DecodeString(cfg.Opc)
	if err != nil {
		return nil, fmt.Errorf("opc: %w", err)
	}
	integ, enc, err := algorithms(cfg.Integrity, cfg.Ciphering)
	if err != nil {
		return nil, err
	}
	digits := cfg.Supi[len("imsi-"):]
	mobileID := new(ie.MobileId5GS)
	if err := mobileID.UnmarshalBinary(util.SupiToBytes(len(cfg.Mcc), len(cfg.Mnc), digits)); err != nil {
		return nil, fmt.Errorf("mobile identity for %s: %w", cfg.Supi, err)
	}
	return &UE{
		cfg: cfg, digits: digits, k: k, opc: opc, integ: integ, enc: enc,
		secCap: securityCapability(enc, integ), mobileID: mobileID,
	}, nil
}

func (u *UE) Supi() string { return u.cfg.Supi }

// RegistrationRequest starts an initial registration with a fresh
// security context, so it is also what a retry sends.
func (u *UE) RegistrationRequest() ([]byte, error) {
	u.secCtx = &message.SecCtx{
		Side: message.UESide, Bearer: message.Bearer3GPP,
		UplinkCount: &message.Count{}, DownlinkCount: &message.Count{},
		CipheringAlg: u.enc, IntegrityAlg: u.integ,
	}
	u.kAmf = nil
	return u.registrationRequest(nil)
}

func (u *UE) registrationRequest(cap5gmm *ie.Capability5GMM) ([]byte, error) {
	return (&message.RegReq{
		RegType5GS:      &ie.RegType5GS{FOR_Pending: true, Value: ie.RegType_InitialReg},
		Ngksi:           &ie.NASKeySetId{Tsc: ie.SecCtxTypeNative, Ksi: ie.NASKeyNA},
		MobileId5GS:     u.mobileID,
		UESecCapability: u.secCap,
		Capability5GMM:  cap5gmm,
	}).MarshalBinary()
}

// PduSessionRequest returns the protected UL NAS Transport carrying a PDU
// Session Establishment Request for PduSessionID. Call after EventRegistered.
func (u *UE) PduSessionRequest() ([]byte, error) {
	if u.kAmf == nil {
		return nil, errors.New("pdu session requested before registration")
	}
	inner, err := (&message.PDUSessEstReq{
		PDUSessId:                      PduSessionID,
		IntegrityProtectionMaxDataRate: &ie.IntegrityProtectionMaxDataRate{Uplink: 0xff, Downlink: 0xff},
		PDUSessType:                    &ie.PDUSessType{Value: ie.PDUSessType_IPv4},
		SSCMode:                        &ie.SSCMode{Mode: ie.SSCMODE1},
		ExtendedProtCfgOpts:            &ie.ExtendedProtCfgOpts{FromMs: &ie.ExtCfgOptFromMs{DNSV4Req: true, DNSV6Req: true}},
	}).MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("encode pdu session establishment request: %w", err)
	}
	ul := &message.ULNASTransport{
		PayloadCntrType: &ie.PayloadCntrType{Value: ie.PayloadCntrType_N1SMInfo},
		PayloadCntr:     &ie.PayloadCntr{Pct: ie.PayloadCntrType_N1SMInfo, Contents: inner},
		PDUSessID:       &ie.PDUSessId2{Value: PduSessionID},
		ReqType:         &ie.ReqType{Value: ie.ReqType_InitialReq},
		DNN:             &ie.DNN{Value: u.cfg.Dnn},
		SNSSAI:          &ie.SNSSAI{SST: uint8(u.cfg.Sst), SD: u.cfg.Sd},
	}
	return message.Marshal(ul, u.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
}

// Handle processes one downlink NAS PDU.
func (u *UE) Handle(pdu []byte) (Result, error) {
	if u.secCtx == nil {
		return Result{}, errors.New("downlink nas before registration request")
	}
	msg, err := message.Parse(pdu, u.secCtx)
	if err != nil {
		return Result{}, fmt.Errorf("decode downlink nas: %w", err)
	}
	switch m := msg.(type) {
	case *message.AuthReq:
		return u.onAuthRequest(m)
	case *message.SecModeCmd:
		return u.onSecurityModeCommand(m)
	case *message.RegAccept:
		reply, err := message.Marshal(&message.RegComplete{}, u.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
		if err != nil {
			return Result{}, fmt.Errorf("encode registration complete: %w", err)
		}
		return Result{Reply: reply, Event: EventRegistered}, nil
	case *message.RegRej:
		return Result{Event: EventRegistrationRejected, Cause: gmmCause(m.Cause5GMM)}, nil
	case *message.AuthRej:
		return Result{Event: EventRegistrationRejected, Cause: "authentication reject"}, nil
	case *message.DLNASTransport:
		return u.onDownlinkTransport(m)
	case *message.CfgUpdateCmd:
		return Result{Event: EventConfigUpdate}, nil
	default:
		// 5GMM Status and the like: nothing the procedures wait for.
		return Result{}, nil
	}
}

func (u *UE) onAuthRequest(m *message.AuthReq) (Result, error) {
	if m.AuthParamRAND5GAuthChlg == nil || m.AuthParamAUTN5GAuthChlg == nil {
		return Result{}, errors.New("authentication request without rand/autn")
	}
	aka, err := runAKA(u.cfg.Supi, u.k, u.opc, m.AuthParamRAND5GAuthChlg.Rand, m.AuthParamAUTN5GAuthChlg.Autn,
		servingNetworkName(u.cfg.Mcc, u.cfg.Mnc))
	if err != nil {
		return Result{}, err
	}
	u.kAmf = aka.kAmf
	if err := u.setNasKeys(u.enc, u.integ); err != nil {
		return Result{}, err
	}
	reply, err := (&message.AuthRsp{AuthRspParam: &ie.AuthRspParam{Res: aka.resStar}}).MarshalBinary()
	if err != nil {
		return Result{}, fmt.Errorf("encode authentication response: %w", err)
	}
	return Result{Reply: reply}, nil
}

func (u *UE) onSecurityModeCommand(m *message.SecModeCmd) (Result, error) {
	if u.kAmf == nil {
		return Result{}, errors.New("security mode command before authentication")
	}
	if m.SelectedNASSecAlgos != nil {
		if err := u.setNasKeys(m.SelectedNASSecAlgos.CipheringAlgo, m.SelectedNASSecAlgos.MsgIntAlgo); err != nil {
			return Result{}, err
		}
	}
	container, err := u.registrationRequest(&ie.Capability5GMM{Length: 1, S1Mode: true, HOAttach: true, LPP: true})
	if err != nil {
		return Result{}, fmt.Errorf("encode registration request container: %w", err)
	}
	imeisv := &ie.MobileId5GS{TypeOfId: ie.IdType_5GS_IMEISV, OddEvenIndic: ie.EvenNumOfIdDigit}
	imeisv.IMEISV[0], imeisv.IMEISV[14], imeisv.IMEISV[15] = 1, 1, 1
	complete := &message.SecModeComplete{IMEISV: imeisv, NASMsgCntr: &ie.NASMsgCntr{Contents: container}}
	reply, err := message.Marshal(complete, u.secCtx, message.SecHdrTypeIntegrityProtectedAndCipheredWithNew5gNasSecCtx)
	if err != nil {
		return Result{}, fmt.Errorf("encode security mode complete: %w", err)
	}
	return Result{Reply: reply}, nil
}

func (u *UE) onDownlinkTransport(m *message.DLNASTransport) (Result, error) {
	if m.PayloadCntr == nil {
		if m.Cause5GMM != nil { // e.g. "payload was not forwarded"
			return Result{Event: EventPduRejected, Cause: gmmCause(m.Cause5GMM)}, nil
		}
		return Result{}, nil
	}
	gsm, err := message.ParseGSM(m.PayloadCntr.Contents)
	if err != nil {
		return Result{}, fmt.Errorf("decode 5gsm payload: %w", err)
	}
	switch g := gsm.(type) {
	case *message.PDUSessEstAccept:
		if g.PDUAddr == nil || len(g.PDUAddr.IPv4) != 4 {
			return Result{Event: EventPduRejected, Cause: "accept without ipv4 address"}, nil
		}
		ip, _ := netip.AddrFromSlice(g.PDUAddr.IPv4)
		return Result{Event: EventPduEstablished, UeIP: ip}, nil
	case *message.PDUSessEstRej:
		return Result{Event: EventPduRejected, Cause: gsmCause(g.Cause5GSM)}, nil
	default:
		return Result{}, nil
	}
}

func (u *UE) setNasKeys(enc ie.AlgCiphering, integ ie.AlgIntegrity) error {
	kenc, kint, err := deriveNasKeys(u.kAmf, enc, integ)
	if err != nil {
		return err
	}
	u.secCtx.CipheringAlg, u.secCtx.IntegrityAlg = enc, integ
	u.secCtx.KnasEnc, u.secCtx.KnasInt = kenc, kint
	return nil
}

func gmmCause(c *ie.Cause5GMM) string {
	if c == nil {
		return "5gmm(unknown)"
	}
	return fmt.Sprintf("5gmm(%d)", c.Value)
}

func gsmCause(c *ie.Cause5GSM) string {
	if c == nil {
		return "5gsm(unknown)"
	}
	return fmt.Sprintf("5gsm(%d)", c.Value)
}

func algorithms(integrity, ciphering string) (ie.AlgIntegrity, ie.AlgCiphering, error) {
	integ := map[string]ie.AlgIntegrity{
		"nia0": message.AlgIntegrity128NIA0, "nia1": message.AlgIntegrity128NIA1,
		"nia2": message.AlgIntegrity128NIA2, "nia3": message.AlgIntegrity128NIA3,
	}
	enc := map[string]ie.AlgCiphering{
		"nea0": message.AlgCiphering128NEA0, "nea1": message.AlgCiphering128NEA1,
		"nea2": message.AlgCiphering128NEA2, "nea3": message.AlgCiphering128NEA3,
	}
	i, ok := integ[integrity]
	if !ok {
		return 0, 0, fmt.Errorf("unknown integrity algorithm %q", integrity)
	}
	e, ok := enc[ciphering]
	if !ok {
		return 0, 0, fmt.Errorf("unknown ciphering algorithm %q", ciphering)
	}
	return i, e, nil
}

func securityCapability(enc ie.AlgCiphering, integ ie.AlgIntegrity) *ie.UESecCapability {
	c := &ie.UESecCapability{Length: 2}
	switch enc {
	case message.AlgCiphering128NEA0:
		c.EA05G = true
	case message.AlgCiphering128NEA1:
		c.EA1_128_5G = true
	case message.AlgCiphering128NEA2:
		c.EA2_128_5G = true
	case message.AlgCiphering128NEA3:
		c.EA3_128_5G = true
	}
	switch integ {
	case message.AlgIntegrity128NIA0:
		c.IA05G = true
	case message.AlgIntegrity128NIA1:
		c.IA1_128_5G = true
	case message.AlgIntegrity128NIA2:
		c.IA2_128_5G = true
	case message.AlgIntegrity128NIA3:
		c.IA3_128_5G = true
	}
	return c
}
