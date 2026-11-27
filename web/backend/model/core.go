package model

import "time"

// LabCoreName is the built-in lab free5GC's name; no saved core network
// may take it.
const LabCoreName = "free5GC"

// Core network kinds: an external core lives outside fru-lab; a free5gc
// core is another free5GC fru-lab deploys on this host, next to the lab's
// own one.
const (
	CoreKindExternal = "external"
	CoreKindFree5gc  = "free5gc"
)

// CoreNetwork is a 5G core besides the lab free5GC that gNBs can attach
// to: where its AMF (N2) and UPF (N3) are, which host interface reaches
// each, and the data networks behind it. A free5gc-kind core gets all of
// that assigned from its slot.
type CoreNetwork struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// external (default; also every core saved before kinds) or free5gc
	Kind string `json:"kind"`
	// kind free5gc: 1-9, picks its networks, container names and
	// webconsole port; WebconsolePort is where its webconsole is published
	Slot           int      `json:"slot,omitempty"`
	WebconsolePort int      `json:"webconsolePort,omitempty"`
	N2Ip           string   `json:"n2Ip"`
	N2Interface    string   `json:"n2Interface,omitempty"`
	N3Ip           string   `json:"n3Ip"`
	N3Interface    string   `json:"n3Interface,omitempty"`
	Dns            []CoreDn `json:"dns"`
	// switched on: the core counts as part of the lab (the Overview draws it)
	Enabled bool `json:"enabled"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// IsFree5gc tells whether n is a free5GC fru-lab deploys.
func (n *CoreNetwork) IsFree5gc() bool {
	return n.Kind == CoreKindFree5gc
}

// CoreDn is one data network behind a core: its DNN and, optionally, the
// address range its UEs get.
type CoreDn struct {
	Name string `json:"name"`
	Cidr string `json:"cidr,omitempty"`
}
