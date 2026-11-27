package model

import "time"

// gNB profile core modes: "lab" dials the free5GC fru-lab deploys over the
// lab's Docker networks; "external" runs the gNB on the host network and
// dials an AMF / UPF anywhere the host can reach.
const (
	GnbCoreLab      = "lab"
	GnbCoreExternal = "external"
)

// GnbProfile is a saved gNB configuration.
type GnbProfile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	GnbID    string `json:"gnbId"`
	GnbName  string `json:"gnbName"`
	Mcc      string `json:"mcc"`
	Mnc      string `json:"mnc"`
	Tac      string `json:"tac"`
	Sst      string `json:"sst"`
	Sd       string `json:"sd"`
	Core     string `json:"core"`
	AmfIp    string `json:"amfIp,omitempty"`
	AmfPort  int    `json:"amfPort,omitempty"`
	UpfIp    string `json:"upfIp,omitempty"`
	UpfPort  int    `json:"upfPort,omitempty"`
	HostIp   string `json:"hostIp,omitempty"`
	LogLevel string `json:"logLevel"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// UeProfile is a saved UE configuration that launches Count UEs, MSIN
// MsinStart, MsinStart+1, ...
type UeProfile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Mcc          string `json:"mcc"`
	Mnc          string `json:"mnc"`
	MsinStart    string `json:"msinStart"`
	Count        int    `json:"count"`
	PermanentKey string `json:"permanentKey"`
	OpValue      string `json:"opValue"`
	Amf          string `json:"amf"`
	Sqn          string `json:"sqn"`
	Dnn          string `json:"dnn"`
	Sst          string `json:"sst"`
	Sd           string `json:"sd"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// RequestDeployGnb picks the profile to deploy the gNB with; an empty
// ProfileID (or no body) uses the lab default.
type RequestDeployGnb struct {
	ProfileID string `json:"profileId"`
}

// ResponseRenderedConfig is a config file as it would be deployed.
type ResponseRenderedConfig struct {
	Yaml string `json:"yaml"`
}

// GnbDeployment is what the gNB is running with, saved next to its
// deployed files.
type GnbDeployment struct {
	ProfileID   string `json:"profileId,omitempty"`
	ProfileName string `json:"profileName"`
	Core        string `json:"core"`
	AmfIp       string `json:"amfIp"`
	HostIp      string `json:"hostIp,omitempty"`
}
