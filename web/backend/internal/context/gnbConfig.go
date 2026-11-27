package context

import (
	"backend/constant"
	"backend/model"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
)

// The lab gNB's fixed addresses on the lab networks (see
// templates/gnb/docker-compose.yaml and the free5gc templates).
const (
	labAmfIp   = "10.0.1.3"
	labUpfIp   = "10.0.1.5"
	labRanN2Ip = "10.0.1.2"
	labRanUeIp = "10.0.2.3"
)

// DefaultGnbProfile is the gNB the lab deploys without a profile: the
// values of templates/gnb/config/gnbcfg.yaml.
func DefaultGnbProfile() model.GnbProfile {
	return model.GnbProfile{
		Name:     "Lab default",
		GnbID:    "000314",
		GnbName:  "gNB",
		Mcc:      "208",
		Mnc:      "93",
		Tac:      "000001",
		Sst:      "1",
		Sd:       "010203",
		Core:     model.GnbCoreLab,
		LogLevel: "info",
	}
}

// gnbAddresses are where the gNB listens and what it dials.
type gnbAddresses struct {
	AmfIp, UpfIp     string
	AmfPort, UpfPort int
	RanN2Ip, RanN3Ip string
	RanUeIp, ApiIp   string
}

func gnbAddressesFor(p *model.GnbProfile) gnbAddresses {
	if p.Core == model.GnbCoreExternal {
		return gnbAddresses{
			AmfIp: p.AmfIp, AmfPort: p.AmfPort, UpfIp: p.UpfIp, UpfPort: p.UpfPort,
			RanN2Ip: p.HostIp, RanN3Ip: p.HostIp, RanUeIp: p.HostIp, ApiIp: p.HostIp,
		}
	}
	return gnbAddresses{
		AmfIp: labAmfIp, AmfPort: 38412, UpfIp: labUpfIp, UpfPort: 2152,
		RanN2Ip: labRanN2Ip, RanN3Ip: labRanN2Ip, RanUeIp: labRanUeIp, ApiIp: labRanN2Ip,
	}
}

var gnbConfigTemplate = template.Must(template.New("gnbcfg").Parse(`gnb:
  amfN2Ip: "{{.A.AmfIp}}"
  ranN2Ip: "{{.A.RanN2Ip}}"
  upfN3Ip: "{{.A.UpfIp}}"
  ranN3Ip: "{{.A.RanN3Ip}}"

  ranControlPlaneIp: "{{.A.RanUeIp}}"
  ranDataPlaneIp: "{{.A.RanUeIp}}"

  amfN2Port: {{.A.AmfPort}}
  ranN2Port: 38413
  upfN3Port: {{.A.UpfPort}}
  ranN3Port: 2152

  ranControlPlanePort: 31413
  ranDataPlanePort: 31414

  gnbId: "{{.P.GnbID}}"
  gnbName: "{{.P.GnbName}}"

  plmnId:
    mcc: "{{.P.Mcc}}"
    mnc: "{{.P.Mnc}}"

  tai:
    tac: "{{.P.Tac}}"
    broadcastPlmnId:
      mcc: "{{.P.Mcc}}"
      mnc: "{{.P.Mnc}}"

  snssai:
    sst: "{{.P.Sst}}"
    sd: "{{.P.Sd}}"

  api:
    ip: "{{.A.ApiIp}}"
    port: 40104

logger:
  level: "{{.P.LogLevel}}" # error, warn, info, debug, trace, test
`))

// externalGnbCompose runs the gNB on the host network, so it reaches any
// core the host can and the UEs reach it at the host's address.
const externalGnbCompose = `services:
  gnb:
    container_name: gnb
    image: alonza0314/free-ran-ue:latest
    command: ./free-ran-ue gnb -c ./gnbcfg.yaml
    network_mode: host
    volumes:
      - ./config/gnbcfg.yaml:/free-ran-ue/gnbcfg.yaml
`

// RenderGnbConfig is the gnbcfg.yaml p deploys.
func RenderGnbConfig(p *model.GnbProfile) (string, error) {
	var buf bytes.Buffer
	err := gnbConfigTemplate.Execute(&buf, struct {
		P *model.GnbProfile
		A gnbAddresses
	}{P: p, A: gnbAddressesFor(p)})
	return buf.String(), err
}

// RenderUeConfig is the uecfg.yaml cfg deploys.
func RenderUeConfig(cfg *UeInstanceConfig) (string, error) {
	raw, err := fs.ReadFile(ueTemplateFS, "templates/ue/config/uecfg.yaml.tmpl")
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("uecfg").Parse(string(raw))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, cfg)
	return buf.String(), err
}

const gnbDeploymentFile = "fru-lab-gnb.json"

// UpGnb deploys the single gNB target with profile p: the template is
// materialized fresh, then its gnbcfg.yaml (and, for an external core, its
// compose file) is replaced with p's.
func (c *composeContext) UpGnb(ctx context.Context, p *model.GnbProfile, profileID string) error {
	target := constant.DEPLOY_TARGET_GNB
	if _, err := c.loadProject(ctx, target, "", true); err != nil {
		return err
	}
	dir := filepath.Join(c.workDir, target)

	cfg, err := RenderGnbConfig(p)
	if err != nil {
		return fmt.Errorf("failed to render gnb config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config", "gnbcfg.yaml"), []byte(cfg), 0644); err != nil {
		return err
	}
	if p.Core == model.GnbCoreExternal {
		if err := os.WriteFile(filepath.Join(dir, "docker-compose.yaml"), []byte(externalGnbCompose), 0644); err != nil {
			return err
		}
	}
	a := gnbAddressesFor(p)
	info, _ := json.Marshal(model.GnbDeployment{ProfileID: profileID, ProfileName: p.Name, Core: p.Core, AmfIp: a.AmfIp, HostIp: p.HostIp})
	if err := os.WriteFile(filepath.Join(dir, gnbDeploymentFile), info, 0644); err != nil {
		return err
	}

	project, err := c.loadProject(ctx, target, "", false)
	if err != nil {
		return err
	}
	c.CtxLog.Infof("Deploying gnb with profile %q (core %s)", p.Name, p.Core)
	return c.up(ctx, project)
}

// GnbDeployment is what the gNB was last deployed with; the lab default
// when it was never deployed through UpGnb.
func (c *composeContext) GnbDeployment() model.GnbDeployment {
	raw, err := os.ReadFile(filepath.Join(c.workDir, constant.DEPLOY_TARGET_GNB, gnbDeploymentFile))
	var d model.GnbDeployment
	if err != nil || json.Unmarshal(raw, &d) != nil {
		def := DefaultGnbProfile()
		return model.GnbDeployment{ProfileName: def.Name, Core: model.GnbCoreLab, AmfIp: labAmfIp}
	}
	return d
}

// GnbRanIp is the address a UE dials to reach the single gNB target.
func (c *composeContext) GnbRanIp() string {
	if d := c.GnbDeployment(); d.Core == model.GnbCoreExternal && d.HostIp != "" {
		return d.HostIp
	}
	return labRanUeIp
}
