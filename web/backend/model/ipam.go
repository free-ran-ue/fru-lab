package model

import "time"

// IpamView is GET /api/ipam: every address pool fru-lab manages with who
// holds what, reconciled with Docker, and what looked wrong doing so.
type IpamView struct {
	Pools    []IpamPool `json:"pools"`
	Warnings []string   `json:"warnings"`
}

// IpamPool is one Docker network's address plan.
type IpamPool struct {
	// cn-ran, sbi, ran-ue, cn-ran-<coreId>, sbi-<coreId>
	ID      string `json:"id"`
	Network string `json:"network"`
	Subnet  string `json:"subnet"`
	Purpose string `json:"purpose"`
	// the core network it belongs to; empty for the lab free5GC's and the
	// shared ones
	CoreID   string         `json:"coreId,omitempty"`
	Reserved []IpamReserved `json:"reserved"`
	Ranges   []IpamRange    `json:"ranges"`
	Leases   []IpamLease    `json:"leases"`
}

// IpamReserved is a fixed address of the plan: the gateway, a core NF's.
type IpamReserved struct {
	Addr string `json:"addr"`
	Role string `json:"role"`
}

// IpamRange is a named range addresses are handed out from.
type IpamRange struct {
	Name  string `json:"name"`
	First string `json:"first"`
	Last  string `json:"last"`
}

// IpamLease is an address someone holds: a lease fru-lab recorded, or a
// container found holding it.
type IpamLease struct {
	Addr  string     `json:"addr"`
	Owner string     `json:"owner"`
	Role  string     `json:"role,omitempty"`
	Since *time.Time `json:"since,omitempty"`
	// a running container holds it right now
	Live bool `json:"live"`
}
