package model

import (
	"encoding/json"
	"time"
)

// gNB profile core modes: "lab" dials the free5GC fru-lab deploys over the
// lab's Docker networks; "external" runs the gNB on the host network and
// dials an AMF / UPF anywhere the host can reach.
const (
	GnbCoreLab      = "lab"
	GnbCoreExternal = "external"
)

// GnbProfile is a saved gNB configuration; every profile is one gNB.
type GnbProfile struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	GnbID   string `json:"gnbId"`
	GnbName string `json:"gnbName"`
	Mcc     string `json:"mcc"`
	Mnc     string `json:"mnc"`
	Tac     string `json:"tac"`
	Sst     string `json:"sst"`
	Sd      string `json:"sd"`
	Core    string `json:"core"`
	AmfIp   string `json:"amfIp,omitempty"`
	AmfPort int    `json:"amfPort,omitempty"`
	UpfIp   string `json:"upfIp,omitempty"`
	UpfPort int    `json:"upfPort,omitempty"`
	// lab core: the gNB's N2/N3 address on cn-ran
	N2Ip string `json:"n2Ip,omitempty"`
	// external core: the host interface and its IPv4 address the gNB
	// listens on
	HostInterface string `json:"hostInterface,omitempty"`
	HostIp        string `json:"hostIp,omitempty"`
	// external core: a saved core network this gNB attaches to; its AMF /
	// UPF addresses are taken from that core on deploy
	CoreNetwork string `json:"coreNetwork,omitempty"`
	LogLevel    string `json:"logLevel"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// UeProfile is a saved UE configuration; every profile is one UE.
type UeProfile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Mcc          string `json:"mcc"`
	Mnc          string `json:"mnc"`
	Msin         string `json:"msin"`
	PermanentKey string `json:"permanentKey"`
	OpValue      string `json:"opValue"`
	Amf          string `json:"amf"`
	Sqn          string `json:"sqn"`
	Dnn          string `json:"dnn"`
	Sst          string `json:"sst"`
	Sd           string `json:"sd"`
	// the gNB profile this UE attaches to unless a deploy says otherwise;
	// empty picks a running gNB by SD
	GnbProfileID string `json:"gnbProfileId,omitempty"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// UnmarshalJSON reads profiles saved before a profile was one UE: they
// had msinStart (and a count, which is dropped) instead of msin.
func (p *UeProfile) UnmarshalJSON(raw []byte) error {
	type plain UeProfile
	var v struct {
		plain
		MsinStart string `json:"msinStart"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	*p = UeProfile(v.plain)
	if p.Msin == "" {
		p.Msin = v.MsinStart
	}
	return nil
}

// ResponseRenderedConfig is a config file as it would be deployed.
type ResponseRenderedConfig struct {
	Yaml string `json:"yaml"`
}

// GnbDeployment is what a gNB is running with, saved next to its deployed
// files.
type GnbDeployment struct {
	ProfileID     string `json:"profileId,omitempty"`
	ProfileName   string `json:"profileName"`
	Core          string `json:"core"`
	AmfIp         string `json:"amfIp"`
	HostIp        string `json:"hostIp,omitempty"`
	HostInterface string `json:"hostInterface,omitempty"`
	GnbID         string `json:"gnbId,omitempty"`
	GnbName       string `json:"gnbName,omitempty"`
	Sd            string `json:"sd,omitempty"`
	// its N2/N3 address (cn-ran, or the host's address on an external
	// core) and where UEs dial it (ran-ue, or the host's address)
	N2Ip  string `json:"n2Ip,omitempty"`
	RanIp string `json:"ranIp,omitempty"`
}

// ProfileConflict is one field of a profile that another saved profile
// already uses.
type ProfileConflict struct {
	Field      string `json:"field"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

type ResponseProfileCheck struct {
	Conflicts []ProfileConflict `json:"conflicts"`
}

// RequestProfileBulk copies profile ProfileID Count times.
type RequestProfileBulk struct {
	ProfileID string `json:"profileId" binding:"required"`
	Count     int    `json:"count" binding:"required"`
}

// HostInterface is a host network interface and its IPv4 addresses.
type HostInterface struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
}
