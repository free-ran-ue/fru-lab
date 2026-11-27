package model

import "time"

type ResponseDeployAction struct {
	Message string `json:"message"`
}

// RequestDeployFree5gc selects which of free5gc's compose templates to
// deploy (e.g. "basic", "ulcl"). Omitting Template (or the whole body)
// defaults to "basic".
type RequestDeployFree5gc struct {
	Template string `json:"template"`
}

type ServiceStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type ResponseDeployStatus struct {
	Target       string          `json:"target"`
	Status       string          `json:"status"`
	Services     []ServiceStatus `json:"services,omitempty"`
	LastDeployed *time.Time      `json:"lastDeployed,omitempty"`
	// Gnb is what a gNB target was deployed with, when known
	Gnb *GnbDeployment `json:"gnb,omitempty"`
}

type ResponseDeployLogs struct {
	Target string   `json:"target"`
	Lines  []string `json:"lines"`
}

// RequestDeployUe carries the subscriber fields needed to template a UE
// instance's config - the frontend fetches the subscriber from webconsole
// itself and derives these, the backend never talks to webconsole directly.
type RequestDeployUe struct {
	Mcc          string `json:"mcc" binding:"required"`
	Mnc          string `json:"mnc" binding:"required"`
	Msin         string `json:"msin" binding:"required"`
	PermanentKey string `json:"permanentKey" binding:"required"`
	OpValue      string `json:"opValue" binding:"required"`
	Amf          string `json:"amf" binding:"required"`
	Sqn          string `json:"sqn" binding:"required"`
	Dnn          string `json:"dnn" binding:"required"`
	Sst          string `json:"sst" binding:"required"`
	Sd           string `json:"sd"`
	// the gNB target to attach to (gnb-<profileId>); empty picks a
	// running gNB serving Sd
	Gnb string `json:"gnb,omitempty"`
}

// RequestDeployUeProfile picks the gNB a UE profile's UE attaches to;
// empty (or no body) lets fru-lab pick.
type RequestDeployUeProfile struct {
	Gnb string `json:"gnb"`
}

type ResponseDeployUeStatus struct {
	Instance string `json:"instance"`
	// the gNB target it was deployed onto; empty when unknown
	Gnb          string          `json:"gnb,omitempty"`
	Status       string          `json:"status"`
	Services     []ServiceStatus `json:"services,omitempty"`
	LastDeployed *time.Time      `json:"lastDeployed,omitempty"`
}

type ResponseDeployUeLogs struct {
	Instance string   `json:"instance"`
	Lines    []string `json:"lines"`
}

type ResponseDeployUeList struct {
	Instances []ResponseDeployUeStatus `json:"instances"`
}

// GnbInstance is the gNB of one deployed gNB profile.
type GnbInstance struct {
	Instance     string          `json:"instance"`
	ProfileID    string          `json:"profileId"`
	Status       string          `json:"status"`
	Services     []ServiceStatus `json:"services,omitempty"`
	LastDeployed *time.Time      `json:"lastDeployed,omitempty"`
	Gnb          GnbDeployment   `json:"gnb"`
}

type ResponseDeployGnbList struct {
	Instances []GnbInstance `json:"instances"`
}
