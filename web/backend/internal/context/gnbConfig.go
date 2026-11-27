package context

import (
	"backend/constant"
	"backend/model"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/compose/v5/pkg/api"
	dockerclient "github.com/moby/moby/client"
)

// A lab-type gNB's N2/N3 address is leased at deploy from the gnb-n2
// range of its core's cn-ran pool (see GnbCnRan), its ran-ue address from
// the gnb-ran range of the ran-ue pool (see ipamPlan.go).

// GnbRanIpPreview is the ran-ue address a config preview shows for a gNB
// that is not deployed (the real one is leased at deploy): the first of
// the gnb-ran range.
var GnbRanIpPreview = func() string {
	r, _ := ranUePool().Range(RangeGnbRan)
	return r.First.String()
}()

// DefaultGnbProfile is the default values of a new gNB profile.
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

// gnbAddressesFor are p's addresses when UEs dial it at ranIp. A lab-type
// gNB listens on p.N2Ip (the assigned address) and dials p.AmfIp / p.UpfIp
// (the lab free5GC's when empty); on an external core everything is the
// host address.
func gnbAddressesFor(p *model.GnbProfile, ranIp string) gnbAddresses {
	if p.Core == model.GnbCoreExternal {
		return gnbAddresses{
			AmfIp: p.AmfIp, AmfPort: p.AmfPort, UpfIp: p.UpfIp, UpfPort: p.UpfPort,
			RanN2Ip: p.HostIp, RanN3Ip: p.HostIp, RanUeIp: p.HostIp, ApiIp: p.HostIp,
		}
	}
	amfIp, upfIp := p.AmfIp, p.UpfIp
	lab := LabCnRan()
	if amfIp == "" {
		amfIp = lab.AmfIp
	}
	if upfIp == "" {
		upfIp = lab.UpfIp
	}
	return gnbAddresses{
		AmfIp: amfIp, AmfPort: 38412, UpfIp: upfIp, UpfPort: 2152,
		RanN2Ip: p.N2Ip, RanN3Ip: p.N2Ip, RanUeIp: ranIp, ApiIp: p.N2Ip,
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

// gnbComposeTemplate is a profile gNB's compose file: on its core's cn-ran
// network and the shared ran-ue network at fixed addresses, or on the host
// network for an external core (so it reaches any core the host can and
// UEs reach it at the host's address).
var gnbComposeTemplate = template.Must(template.New("compose").Parse(`services:
  gnb:
    container_name: {{.Container}}
    image: alonza0314/free-ran-ue:latest
    command: ./free-ran-ue gnb -c ./gnbcfg.yaml
    volumes:
      - ./config/gnbcfg.yaml:/free-ran-ue/gnbcfg.yaml
{{- if .External}}
    network_mode: host
{{- else}}
    networks:
      cn-ran:
        ipv4_address: {{.N2Ip}}
      ran-ue:
        ipv4_address: {{.RanIp}}

networks:
  cn-ran:
    external: true
    name: {{.CnRan}}
  ran-ue:
    external: true
    name: ` + RanUeNetwork + `
{{- end}}
`))

// RenderGnbConfig is the gnbcfg.yaml p deploys when UEs dial it at ranIp
// (ignored on an external core).
func RenderGnbConfig(p *model.GnbProfile, ranIp string) (string, error) {
	var buf bytes.Buffer
	err := gnbConfigTemplate.Execute(&buf, struct {
		P *model.GnbProfile
		A gnbAddresses
	}{P: p, A: gnbAddressesFor(p, ranIp)})
	return buf.String(), err
}

// renderGnbCompose is the compose file of p's gNB as container; a lab-type
// one joins cn-ran network cnRan.
func renderGnbCompose(container string, p *model.GnbProfile, ranIp, cnRan string) (string, error) {
	var buf bytes.Buffer
	err := gnbComposeTemplate.Execute(&buf, struct {
		Container          string
		External           bool
		N2Ip, RanIp, CnRan string
	}{Container: container, External: p.Core == model.GnbCoreExternal, N2Ip: p.N2Ip, RanIp: ranIp, CnRan: cnRan})
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

var (
	gnbInstanceTarget  = regexp.MustCompile(`^gnb-[0-9a-f]{12}$`)
	gnbInstanceProject = regexp.MustCompile(`^frulab-gnb-[0-9a-f]{12}$`)
)

// IsGnbInstance tells whether target is a profile gNB, gnb-<profile id>.
func IsGnbInstance(target string) bool {
	return gnbInstanceTarget.MatchString(target)
}

// GnbInstanceTarget is the target (and container) name of the gNB of
// profile profileID.
func GnbInstanceTarget(profileID string) string {
	return constant.GNB_INSTANCE_PREFIX + profileID
}

func gnbInstanceProjectName(target string) string {
	return "frulab-" + target
}

func (c *composeContext) gnbInstanceDir(target string) string {
	return filepath.Join(c.workDir, target)
}

func (c *composeContext) gnbInstanceMaterialized(target string) bool {
	_, err := os.Stat(filepath.Join(c.gnbInstanceDir(target), "docker-compose.yaml"))
	return err == nil
}

// loadGnbInstanceProject loads a deployed profile gNB's project from the
// files UpGnbInstance rendered.
func (c *composeContext) loadGnbInstanceProject(ctx context.Context, target string) (*types.Project, error) {
	c.materializeMu.Lock()
	defer c.materializeMu.Unlock()
	if !c.gnbInstanceMaterialized(target) {
		return nil, fmt.Errorf("%s is not deployed", target)
	}
	dir := c.gnbInstanceDir(target)
	project, err := loader.LoadWithContext(ctx, types.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "docker-compose.yaml")}},
	}, func(o *loader.Options) {
		o.SetProjectName(gnbInstanceProjectName(target), true)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load compose project for %s: %v", target, err)
	}
	stampComposeLabels(project)
	return project, nil
}

// LockGnbDeploys serializes profile gNB deploys; the caller checks what
// the gNB may use and calls UpGnbInstance before unlocking.
func (c *composeContext) LockGnbDeploys() func() {
	c.gnbDeployMu.Lock()
	return c.gnbDeployMu.Unlock
}

// UpGnbInstance deploys the gNB of profile p (which must have an ID) into
// <workDir>/gnb-<id>. A lab-type gNB joins cn-ran pool on (the lab
// free5GC's when nil) at its leased address of the gnb-n2 range there
// (see ipamContext.Allocate: the same one as last time, else the lowest
// free one), dials that core's AMF / UPF, and gets its leased address of
// the gnb-ran range of ran-ue; an external-core one runs on the host
// network. A failed deploy leaves nothing behind but its leases. The
// caller holds LockGnbDeploys.
func (c *composeContext) UpGnbInstance(ctx context.Context, prof *model.GnbProfile, on *GnbCnRan) (*model.GnbDeployment, error) {
	p := *prof
	target := GnbInstanceTarget(p.ID)
	cnRan := ""
	d := &model.GnbDeployment{
		ProfileID: p.ID, ProfileName: p.Name, Core: p.Core, AmfIp: p.AmfIp,
		HostIp: p.HostIp, HostInterface: p.HostInterface,
		GnbID: p.GnbID, GnbName: p.GnbName, Sd: p.Sd, N2Ip: p.HostIp, RanIp: p.HostIp,
		CoreNetwork: p.CoreNetwork,
	}
	if p.Core != model.GnbCoreExternal {
		if on == nil {
			lab := LabCnRan()
			on = &lab
		}
		ranIp, err := c.ipam.Allocate(ctx, ranUePool(), RangeGnbRan, target, LeaseRoleRan)
		if err != nil {
			return nil, err
		}
		n2Ip, err := c.ipam.Allocate(ctx, on.pool, RangeGnbN2, target, LeaseRoleN2)
		if err != nil {
			return nil, err
		}
		p.N2Ip, p.AmfIp, p.UpfIp = n2Ip.String(), on.AmfIp, on.UpfIp
		p.HostIp, p.HostInterface = "", ""
		cnRan = on.Network
		d.AmfIp, d.HostIp, d.HostInterface = on.AmfIp, "", ""
		d.N2Ip, d.RanIp, d.CoreNetwork = p.N2Ip, ranIp.String(), on.CoreNetwork
	}

	if err := c.materializeGnbInstance(target, &p, d, cnRan); err != nil {
		c.removeGnbInstanceDir(target)
		return nil, err
	}
	project, err := c.loadGnbInstanceProject(ctx, target)
	if err != nil {
		c.removeGnbInstanceDir(target)
		return nil, err
	}
	c.CtxLog.Infof("Deploying %s with profile %q (core %s %s, n2 %s, ran-ue %s)", target, p.Name, p.Core, d.CoreNetwork, d.N2Ip, d.RanIp)
	if err := c.up(ctx, project); err != nil {
		c.removeGnbInstanceDir(target)
		return nil, err
	}
	return d, nil
}

func (c *composeContext) materializeGnbInstance(target string, p *model.GnbProfile, d *model.GnbDeployment, cnRan string) error {
	cfg, err := RenderGnbConfig(p, d.RanIp)
	if err != nil {
		return fmt.Errorf("failed to render gnb config: %v", err)
	}
	compose, err := renderGnbCompose(target, p, d.RanIp, cnRan)
	if err != nil {
		return fmt.Errorf("failed to render gnb compose file: %v", err)
	}
	info, err := json.Marshal(d)
	if err != nil {
		return err
	}

	c.materializeMu.Lock()
	defer c.materializeMu.Unlock()
	dir := c.gnbInstanceDir(target)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to clean working dir %s: %v", dir, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0755); err != nil {
		return fmt.Errorf("failed to create working dir %s: %v", dir, err)
	}
	for name, content := range map[string][]byte{
		"config/gnbcfg.yaml":  []byte(cfg),
		"docker-compose.yaml": []byte(compose),
		gnbDeploymentFile:     info,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0644); err != nil {
			return err
		}
	}
	return nil
}

func (c *composeContext) removeGnbInstanceDir(target string) {
	c.materializeMu.Lock()
	defer c.materializeMu.Unlock()
	if err := os.RemoveAll(c.gnbInstanceDir(target)); err != nil {
		c.CtxLog.Errorf("Failed to remove working dir of %s: %v", target, err)
	}
}

// DownGnbInstance stops the gNB of profile profileID and removes its
// working dir, so it no longer is listed.
func (c *composeContext) DownGnbInstance(ctx context.Context, profileID string) error {
	target := GnbInstanceTarget(profileID)
	opts := api.DownOptions{RemoveOrphans: true}
	if project, err := c.loadGnbInstanceProject(ctx, target); err == nil {
		opts.Project = project
	}
	c.CtxLog.Infof("Stopping %s", target)
	if err := c.service.Down(ctx, gnbInstanceProjectName(target), opts); err != nil {
		return err
	}
	c.removeGnbInstanceDir(target)
	return nil
}

// ListGnbInstances is the profile IDs of the deployed profile gNBs.
func (c *composeContext) ListGnbInstances() ([]string, error) {
	entries, err := os.ReadDir(c.workDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list gnb instances: %v", err)
	}
	ids := make([]string, 0)
	for _, e := range entries {
		if e.IsDir() && IsGnbInstance(e.Name()) && c.gnbInstanceMaterialized(e.Name()) {
			ids = append(ids, strings.TrimPrefix(e.Name(), constant.GNB_INSTANCE_PREFIX))
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// GnbInstanceDeployment is what the gNB of profile profileID was deployed
// with; nil when it is not deployed.
func (c *composeContext) GnbInstanceDeployment(profileID string) *model.GnbDeployment {
	raw, err := os.ReadFile(filepath.Join(c.gnbInstanceDir(GnbInstanceTarget(profileID)), gnbDeploymentFile))
	if err != nil {
		return nil
	}
	var d model.GnbDeployment
	if json.Unmarshal(raw, &d) != nil {
		return nil
	}
	return &d
}

// NetworkAddresses maps the IPv4 address of every container attached to
// docker network name to the container's name; empty when the network
// does not exist.
func (c *composeContext) NetworkAddresses(ctx context.Context, name string) (map[netip.Addr]string, error) {
	if c.inspectNetworkFn != nil {
		return c.inspectNetworkFn(ctx, name)
	}
	res, err := c.dockerCli.Client().NetworkInspect(ctx, name, dockerclient.NetworkInspectOptions{})
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return map[netip.Addr]string{}, nil
		}
		return nil, fmt.Errorf("failed to inspect network %s: %v", name, err)
	}
	out := make(map[netip.Addr]string, len(res.Network.Containers))
	for _, ep := range res.Network.Containers {
		if ep.IPv4Address.IsValid() {
			out[ep.IPv4Address.Addr()] = ep.Name
		}
	}
	return out, nil
}

// gnbLeaseRecords are the addresses of every deployed lab-type profile
// gNB as leases, for IPAM to adopt the ones fru-lab handed out before it
// recorded leases.
func (c *composeContext) gnbLeaseRecords() []IpamLease {
	ids, err := c.ListGnbInstances()
	if err != nil {
		return nil
	}
	var out []IpamLease
	for _, id := range ids {
		d := c.GnbInstanceDeployment(id)
		if d == nil || d.Core == model.GnbCoreExternal {
			continue
		}
		target := GnbInstanceTarget(id)
		pool := PoolCnRan
		if d.CoreNetwork != "" {
			pool = PoolCnRan + "-" + d.CoreNetwork
		}
		if d.N2Ip != "" {
			out = append(out, IpamLease{Pool: pool, Addr: d.N2Ip, Owner: target, Role: LeaseRoleN2})
		}
		if d.RanIp != "" {
			out = append(out, IpamLease{Pool: PoolRanUe, Addr: d.RanIp, Owner: target, Role: LeaseRoleRan})
		}
	}
	return out
}
