package model

import "time"

// LabCoreName is the built-in lab free5GC's name; no saved core network
// may take it.
const LabCoreName = "free5GC"

// CoreNetwork is a 5G core outside the lab that gNBs can attach to: where
// its AMF (N2) and UPF (N3) are, which host interface reaches each, and the
// data networks behind it.
type CoreNetwork struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	N2Ip        string   `json:"n2Ip"`
	N2Interface string   `json:"n2Interface,omitempty"`
	N3Ip        string   `json:"n3Ip"`
	N3Interface string   `json:"n3Interface,omitempty"`
	Dns         []CoreDn `json:"dns"`
	// switched on: the core counts as part of the lab (the Overview draws it)
	Enabled bool `json:"enabled"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// CoreDn is one data network behind a core: its DNN and, optionally, the
// address range its UEs get.
type CoreDn struct {
	Name string `json:"name"`
	Cidr string `json:"cidr,omitempty"`
}
