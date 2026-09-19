package model

import "time"

type ResponseDeployAction struct {
	Message string `json:"message"`
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
}

type ResponseDeployLogs struct {
	Target string   `json:"target"`
	Lines  []string `json:"lines"`
}
