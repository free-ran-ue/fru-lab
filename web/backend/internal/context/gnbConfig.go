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

// The lab core's AMF / UPF addresses on cn-ran (see the free5gc
// templates), and the shared networks a lab-core gNB joins.
const (
	labAmfIp = "10.0.1.3"
	labUpfIp = "10.0.1.5"

	CnRanNetwork = "frulab-cn-ran"
	RanUeNetwork = "frulab-ran-ue"
)

// A lab-core gNB profile's N2/N3 address (n2Ip) is in GnbN2IpFirst ..
// GnbN2IpLast; the lower cn-ran addresses are kept for the gateway and the
// core's NFs. Its ran-ue address is picked at deploy from
// gnbRanIpFirst .. gnbRanIpLast.
var (
	GnbN2IpFirst  = netip.MustParseAddr("10.0.1.20")
	GnbN2IpLast   = netip.MustParseAddr("10.0.1.254")
	gnbRanIpFirst = netip.MustParseAddr("10.0.2.200")
	gnbRanIpLast  = netip.MustParseAddr("10.0.2.250")
)

// GnbRanIpPreview is the ran-ue address a config preview shows for a gNB
// that is not deployed (the real one is picked at deploy).
var GnbRanIpPreview = gnbRanIpFirst.String()

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
		N2Ip:     GnbN2IpFirst.String(),
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

// gnbAddressesFor are p's addresses when UEs dial it at ranIp (lab core;
// on an external core everything is the host address).
func gnbAddressesFor(p *model.GnbProfile, ranIp string) gnbAddresses {
	if p.Core == model.GnbCoreExternal {
		return gnbAddresses{
			AmfIp: p.AmfIp, AmfPort: p.AmfPort, UpfIp: p.UpfIp, UpfPort: p.UpfPort,
			RanN2Ip: p.HostIp, RanN3Ip: p.HostIp, RanUeIp: p.HostIp, ApiIp: p.HostIp,
		}
	}
	return gnbAddresses{
		AmfIp: labAmfIp, AmfPort: 38412, UpfIp: labUpfIp, UpfPort: 2152,
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

// gnbComposeTemplate is a profile gNB's compose file: on the lab networks
// at fixed addresses, or on the host network for an external core (so it
// reaches any core the host can and UEs reach it at the host's address).
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
    name: ` + CnRanNetwork + `
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

func renderGnbCompose(container string, p *model.GnbProfile, ranIp string) (string, error) {
	var buf bytes.Buffer
	err := gnbComposeTemplate.Execute(&buf, struct {
		Container   string
		External    bool
		N2Ip, RanIp string
	}{Container: container, External: p.Core == model.GnbCoreExternal, N2Ip: p.N2Ip, RanIp: ranIp})
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
// <workDir>/gnb-<id>. On the lab core its ran-ue address is the lowest
// free one of the gNB range. A failed deploy leaves nothing behind. The
// caller holds LockGnbDeploys.
func (c *composeContext) UpGnbInstance(ctx context.Context, p *model.GnbProfile) (*model.GnbDeployment, error) {
	target := GnbInstanceTarget(p.ID)
	ranIp, n2Ip, amfIp := p.HostIp, p.HostIp, p.AmfIp
	if p.Core != model.GnbCoreExternal {
		used, err := c.usedRanUeIps(ctx, target)
		if err != nil {
			return nil, err
		}
		addr, ok := PickFreeAddr(gnbRanIpFirst, gnbRanIpLast, func(a netip.Addr) bool { return used[a] })
		if !ok {
			return nil, fmt.Errorf("no free ran-ue address left in %s - %s", gnbRanIpFirst, gnbRanIpLast)
		}
		ranIp, n2Ip, amfIp = addr.String(), p.N2Ip, labAmfIp
	}

	d := &model.GnbDeployment{
		ProfileID: p.ID, ProfileName: p.Name, Core: p.Core, AmfIp: amfIp,
		HostIp: p.HostIp, HostInterface: p.HostInterface,
		GnbID: p.GnbID, GnbName: p.GnbName, Sd: p.Sd, N2Ip: n2Ip, RanIp: ranIp,
	}
	if err := c.materializeGnbInstance(target, p, d); err != nil {
		c.removeGnbInstanceDir(target)
		return nil, err
	}
	project, err := c.loadGnbInstanceProject(ctx, target)
	if err != nil {
		c.removeGnbInstanceDir(target)
		return nil, err
	}
	c.CtxLog.Infof("Deploying %s with profile %q (core %s, n2 %s, ran-ue %s)", target, p.Name, p.Core, n2Ip, ranIp)
	if err := c.up(ctx, project); err != nil {
		c.removeGnbInstanceDir(target)
		return nil, err
	}
	return d, nil
}

func (c *composeContext) materializeGnbInstance(target string, p *model.GnbProfile, d *model.GnbDeployment) error {
	cfg, err := RenderGnbConfig(p, d.RanIp)
	if err != nil {
		return fmt.Errorf("failed to render gnb config: %v", err)
	}
	compose, err := renderGnbCompose(target, p, d.RanIp)
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

// usedRanUeIps are the ran-ue addresses a new gNB must not take: every
// container's on the network, and every other deployed profile gNB's.
func (c *composeContext) usedRanUeIps(ctx context.Context, exceptTarget string) (map[netip.Addr]bool, error) {
	attached, err := c.NetworkAddresses(ctx, RanUeNetwork)
	if err != nil {
		return nil, err
	}
	used := make(map[netip.Addr]bool, len(attached))
	for a, name := range attached {
		if name != exceptTarget {
			used[a] = true
		}
	}
	ids, err := c.ListGnbInstances()
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if GnbInstanceTarget(id) == exceptTarget {
			continue
		}
		if d := c.GnbInstanceDeployment(id); d != nil {
			if a, err := netip.ParseAddr(d.RanIp); err == nil {
				used[a] = true
			}
		}
	}
	return used, nil
}

// PickFreeAddr is the lowest address in first .. last that is not used.
func PickFreeAddr(first, last netip.Addr, used func(netip.Addr) bool) (netip.Addr, bool) {
	for a := first; a.IsValid() && a.Compare(last) <= 0; a = a.Next() {
		if !used(a) {
			return a, true
		}
	}
	return netip.Addr{}, false
}
