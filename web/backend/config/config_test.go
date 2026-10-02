package config

import (
	"testing"
	"time"
)

func TestDeployDefaults(t *testing.T) {
	var d DeployIE
	if d.WebconsolePort() != 5000 || d.WebconsoleURL() != "http://localhost:5000" || d.DeployTimeout() != 5*time.Minute {
		t.Fatalf("defaults: port %d url %s timeout %s", d.WebconsolePort(), d.WebconsoleURL(), d.DeployTimeout())
	}
	d = DeployIE{Timeout: 90 * time.Second, Webconsole: WebconsoleIE{Port: 5055, Host: "host.docker.internal"}}
	if d.WebconsolePort() != 5055 || d.WebconsoleURL() != "http://host.docker.internal:5055" || d.DeployTimeout() != 90*time.Second {
		t.Fatalf("set: port %d url %s timeout %s", d.WebconsolePort(), d.WebconsoleURL(), d.DeployTimeout())
	}
}
