// Package profile holds the user-facing description of a throughput test
// and turns it into the concrete per-gNB settings a run needs.
package profile

import "net/netip"

// Profile is what the setup page edits and what a run starts from. Field
// names are the JSON contract shared with fru-lab's frontend.
type Profile struct {
	Name    string      `json:"name"`
	Scale   Scale       `json:"scale"`
	Gnb     GnbTemplate `json:"gnb"`
	Ue      UeTemplate  `json:"ue"`
	Traffic Traffic     `json:"traffic"`
	Network Network     `json:"network"`
	Rates   Rates       `json:"rates"`
}

// MaxPacketSize is the largest inner packet a profile may ask for, for
// jumbo-frame networks; the real limit is the interfaces' MTU minus
// GtpOverhead, which the run checks against the host.
const MaxPacketSize = 9000

// GtpOverhead is what N3 adds to an inner packet: outer IPv4 (20), UDP
// (8), GTP-U (8) and, on downlink from gtp5g, the optional fields and a
// PDU Session Container (8).
const GtpOverhead = 44

// Traffic is what every established UE sends and receives, at a fixed
// rate (design Q10). PacketSize is the inner IP packet's length; Port is
// the UDP port used at the N6 sink and at the (simulated) UEs.
type Traffic struct {
	UlMbps     float64 `json:"ulMbps"` // per UE; 0 = no uplink
	DlMbps     float64 `json:"dlMbps"` // per UE; 0 = no downlink
	PacketSize int     `json:"packetSize"`
	Port       int     `json:"port"`
	// MaxDurationMin stops the run this many minutes after it started,
	// exactly like pressing Stop; 0 = run until Stop (design N6).
	MaxDurationMin int `json:"maxDurationMin"`
	// DlBatchMs sends each UE this many milliseconds of downlink in a row,
	// so UDP GSO can send them as one; 0 = one packet per UE in turn.
	DlBatchMs int `json:"dlBatchMs"`
}

type Scale struct {
	GnbCount int `json:"gnbCount"`
	UeCount  int `json:"ueCount"`
}

// GnbTemplate is expanded once per gNB. GnbIDStart is hex and keeps its
// width when incremented; NamePattern must contain "{i}" (1-based index).
type GnbTemplate struct {
	GnbIDStart  string `json:"gnbIdStart"`
	NamePattern string `json:"namePattern"`
	Mcc         string `json:"mcc"`
	Mnc         string `json:"mnc"`
	Tac         string `json:"tac"`
	Sst         int    `json:"sst"`
	Sd          string `json:"sd"`
}

// UeTemplate is expanded once per UE. MsinStart is decimal and keeps its
// width when incremented; MCC+MNC+MSIN must be 15 digits. The UE uses the
// gNB template's PLMN. Key/Opc/Amf/Sqn must match the subscriber data the
// user created in the core (the tester does not provision subscribers).
type UeTemplate struct {
	MsinStart string `json:"msinStart"`
	Key       string `json:"key"`
	Opc       string `json:"opc"`
	Amf       string `json:"amf"`
	Sqn       string `json:"sqn"`
	Integrity string `json:"integrity"` // nia0..nia3
	Ciphering string `json:"ciphering"` // nea0..nea3
	Dnn       string `json:"dnn"`
	Sst       int    `json:"sst"`
	Sd        string `json:"sd"`
}

type Network struct {
	N2 N2Network `json:"n2"`
	N3 N3Network `json:"n3"`
	N6 N6Network `json:"n6"`
}

// N6Network is the data-network side. Uplink leaves the UPF addressed to
// SinkIP, an address with its prefix length such as 172.26.6.1/16 (added
// to Interface as it is if the host does not have it, so the host gets a
// route to the UPF's N6 subnet); downlink is sent from it to the UEs'
// IPs, which the tester routes via UpfIP
// (UePool via UpfIP dev Interface) for the duration of the run (Q16).
type N6Network struct {
	Interface string `json:"interface"`
	SinkIP    string `json:"sinkIp"`
	UpfIP     string `json:"upfIp"`
	UePool    string `json:"uePool"`
}

// Sink is SinkIP parsed; call it only on a validated profile.
func (n N6Network) Sink() netip.Prefix { return netip.MustParsePrefix(n.SinkIP) }

// N2Network gives each gNB its own local IP from Cidr, starting at StartIP.
type N2Network struct {
	Interface string `json:"interface"`
	Cidr      string `json:"cidr"`
	StartIP   string `json:"startIp"`
	AmfIP     string `json:"amfIp"`
	AmfPort   int    `json:"amfPort"`
}

// N3Network gives each gNB its own N3 IP from Cidr (design N1); the data
// plane sends uplink from it and the UPF sends downlink to it.
type N3Network struct {
	Interface string `json:"interface"`
	Cidr      string `json:"cidr"`
	StartIP   string `json:"startIp"`
	UpfIP     string `json:"upfIp"`
	UpfPort   int    `json:"upfPort"`
}

type Rates struct {
	N2           StageRate     `json:"n2"`
	Registration ProcedureRate `json:"registration"`
	Pdu          ProcedureRate `json:"pdu"`
	// Deregistration paces the UE deregistrations that Stop runs; the core
	// releases each UE's PDU session with it.
	Deregistration ProcedureRate `json:"deregistration"`
}

// ProcedureRate paces a per-UE stage: at most RatePerSec new attempts per
// second (token bucket) and at most MaxInFlight attempts at once
// (semaphore), each bounded by TimeoutMs; Retries as in StageRate.
type ProcedureRate struct {
	RatePerSec  int `json:"ratePerSec"`
	MaxInFlight int `json:"maxInFlight"`
	TimeoutMs   int `json:"timeoutMs"`
	Retries     int `json:"retries"`
}

// StageRate bounds one attempt with TimeoutMs; Retries is how many more
// attempts a failed item gets (it is requeued at the back each time).
type StageRate struct {
	TimeoutMs int `json:"timeoutMs"`
	Retries   int `json:"retries"`
}
