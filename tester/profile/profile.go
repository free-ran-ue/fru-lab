// Package profile holds the user-facing description of a throughput test
// and turns it into the concrete per-gNB settings a run needs.
package profile

// Profile is what the setup page edits and what a run starts from. Field
// names are the JSON contract shared with fru-lab's frontend.
type Profile struct {
	Name    string      `json:"name"`
	Scale   Scale       `json:"scale"`
	Gnb     GnbTemplate `json:"gnb"`
	Network Network     `json:"network"`
	Rates   Rates       `json:"rates"`
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

type Network struct {
	N2 N2Network `json:"n2"`
	N3 N3Network `json:"n3"`
}

// N2Network gives each gNB its own local IP from Cidr, starting at StartIP.
type N2Network struct {
	Interface string `json:"interface"`
	Cidr      string `json:"cidr"`
	StartIP   string `json:"startIp"`
	AmfIP     string `json:"amfIp"`
	AmfPort   int    `json:"amfPort"`
}

// N3Network is validated and allocated now so the setup page can show the
// full per-gNB plan, but nothing binds to these IPs until phase 3.
type N3Network struct {
	Interface string `json:"interface"`
	Cidr      string `json:"cidr"`
	StartIP   string `json:"startIp"`
	UpfIP     string `json:"upfIp"`
	UpfPort   int    `json:"upfPort"`
}

type Rates struct {
	N2 StageRate `json:"n2"`
}

// StageRate bounds one attempt with TimeoutMs; Retries is how many more
// attempts a failed item gets (it is requeued at the back each time).
type StageRate struct {
	TimeoutMs int `json:"timeoutMs"`
	Retries   int `json:"retries"`
}
