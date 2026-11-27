package context

import (
	"backend/constant"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v5/pkg/api"
)

// A free5gc-kind core network is deployed as target core-<core id>
// (compose project frulab-core-<core id>, working dir <workDir>/core-<id>)
// from the same templates as the lab free5GC, rendered with its slot's
// IPAM pools (see coreCnRanPool / coreSbiPool): its cn-ran network is
// frulab-cn-ran-<slot> (10.0.(100+slot).0/24, AMF .3, UPF .5), its SBI
// network 10.0.(110+slot).0/24, and every host-global name (container
// names c<slot>-*, bridge names) carries the slot.
const (
	CoreInstancePrefix = "core-"
	MaxCoreSlot        = 9

	coreDeploymentFile = "fru-lab-core.json"
	coreCnRanPrefix    = CnRanNetwork + "-"
	// Linux interface names are at most 15 characters
	maxIfNameLen = 15
)

var (
	coreInstanceTarget  = regexp.MustCompile(`^core-[0-9a-f]{12}$`)
	coreInstanceProject = regexp.MustCompile(`^frulab-core-[0-9a-f]{12}$`)
	coreCnRanName       = regexp.MustCompile(`^frulab-cn-ran-([1-9])$`)
)

// IsCoreInstance tells whether target is a free5gc-kind core network,
// core-<core id>.
func IsCoreInstance(target string) bool {
	return coreInstanceTarget.MatchString(target)
}

// CoreInstanceTarget is the deploy target of core network coreID.
func CoreInstanceTarget(coreID string) string {
	return CoreInstancePrefix + coreID
}

// coreIDOf is the core network ID of core target core-<core id>.
func coreIDOf(target string) string {
	return strings.TrimPrefix(target, CoreInstancePrefix)
}

func coreInstanceProjectName(target string) string {
	return "frulab-" + target
}

// CoreCnRanNetwork is the docker network the core in slot slot shares with
// its gNBs.
func CoreCnRanNetwork(slot int) string {
	return fmt.Sprintf("%s%d", coreCnRanPrefix, slot)
}

// coreDeployment is what a core was last deployed with, saved next to its
// files.
type coreDeployment struct {
	Template       string `json:"template"`
	Slot           int    `json:"slot"`
	WebconsolePort int    `json:"webconsolePort"`
}

// materializeCoreFiles renders template tmpl for the core whose networks
// are the pools cnRan and sbi into dir (which must not exist yet or be
// empty): every *.tmpl file gets the pools' subnets, the NFs' addresses by
// role, the network and bridge names and the container name prefix (see
// coreRender); everything else a compose project owns (its own networks,
// volumes) is already scoped by the project name, and the webconsole's
// host port is set on load (publishWebconsole).
func materializeCoreFiles(dir string, tmpl composeTemplate, cnRan, sbi IpamPool) error {
	src, err := fs.Sub(tmpl.templateFS, tmpl.templateDir)
	if err != nil {
		return err
	}
	render, err := newCoreRender(cnRan, sbi)
	if err != nil {
		return err
	}
	if err := renderTemplateFS(src, dir, render, render.funcs()); err != nil {
		return err
	}
	// go:embed drops the executable bit
	return restoreScriptPermissions(dir)
}

func (c *composeContext) coreInstanceDir(target string) string {
	return filepath.Join(c.workDir, target)
}

func (c *composeContext) coreInstanceMaterialized(target string) bool {
	_, err := os.Stat(filepath.Join(c.coreInstanceDir(target), "docker-compose.yaml"))
	return err == nil
}

// materializeCoreInstance writes the files of core target, deployed as d,
// into its (fresh) working dir.
func (c *composeContext) materializeCoreInstance(target string, d coreDeployment) error {
	info, err := json.Marshal(d)
	if err != nil {
		return err
	}
	tmpl := c.targets[constant.DEPLOY_TARGET_FREE5GC].template(d.Template)

	c.materializeMu.Lock()
	defer c.materializeMu.Unlock()
	dir := c.coreInstanceDir(target)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to clean working dir %s: %v", dir, err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create working dir %s: %v", dir, err)
	}
	if err := materializeCoreFiles(dir, tmpl, coreCnRanPool(coreIDOf(target), d.Slot), coreSbiPool(coreIDOf(target), d.Slot)); err != nil {
		return fmt.Errorf("failed to materialize %s: %v", target, err)
	}
	return os.WriteFile(filepath.Join(dir, coreDeploymentFile), info, 0644)
}

func (c *composeContext) coreDeploymentOf(target string) (coreDeployment, bool) {
	var d coreDeployment
	raw, err := os.ReadFile(filepath.Join(c.coreInstanceDir(target), coreDeploymentFile))
	if err != nil || json.Unmarshal(raw, &d) != nil {
		return coreDeployment{}, false
	}
	return d, true
}

// CoreInstanceTemplate is the template core network coreID was last
// deployed from; "" when it never was.
func (c *composeContext) CoreInstanceTemplate(coreID string) string {
	d, _ := c.coreDeploymentOf(CoreInstanceTarget(coreID))
	return d.Template
}

// loadCoreInstanceProject loads a core's project from the files
// UpCoreInstance wrote, its webconsole published on the recorded port.
func (c *composeContext) loadCoreInstanceProject(ctx context.Context, target string) (*types.Project, error) {
	c.materializeMu.Lock()
	defer c.materializeMu.Unlock()
	if !c.coreInstanceMaterialized(target) {
		return nil, fmt.Errorf("%s is not deployed", target)
	}
	dir := c.coreInstanceDir(target)
	project, err := loader.LoadWithContext(ctx, types.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "docker-compose.yaml")}},
	}, func(o *loader.Options) {
		o.SetProjectName(coreInstanceProjectName(target), true)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load compose project for %s: %v", target, err)
	}
	stampComposeLabels(project)
	if d, ok := c.coreDeploymentOf(target); ok && d.WebconsolePort > 0 {
		publishWebconsole(project, d.WebconsolePort)
	}
	return project, nil
}

// UpCoreInstance deploys core network coreID, in slot slot, from template
// variant with its webconsole on host port webconsolePort. Its cn-ran
// network is created first unless it exists.
func (c *composeContext) UpCoreInstance(ctx context.Context, coreID string, slot, webconsolePort int, variant string) error {
	if slot < 1 || slot > MaxCoreSlot {
		return fmt.Errorf("core network slot %d is not 1 - %d", slot, MaxCoreSlot)
	}
	target := CoreInstanceTarget(coreID)
	if err := c.ensureNetwork(ctx, coreCnRanPool(coreID, slot)); err != nil {
		return err
	}
	// its compose project creates its SBI network
	if err := c.ipam.checkOverlaps(ctx, coreSbiPool(coreID, slot)); err != nil {
		return err
	}
	if err := c.materializeCoreInstance(target, coreDeployment{Template: variant, Slot: slot, WebconsolePort: webconsolePort}); err != nil {
		return err
	}
	project, err := c.loadCoreInstanceProject(ctx, target)
	if err != nil {
		return err
	}
	c.CtxLog.Infof("Deploying %s (project %s, template %s, slot %d, webconsole port %d)", target, project.Name, variant, slot, webconsolePort)
	return c.up(ctx, project)
}

// downCoreInstance stops core target; its files (and its cn-ran network)
// stay, so it is listed as stopped.
func (c *composeContext) downCoreInstance(ctx context.Context, target string) error {
	opts := api.DownOptions{RemoveOrphans: true}
	if project, err := c.loadCoreInstanceProject(ctx, target); err == nil {
		opts.Project = project
	}
	c.CtxLog.Infof("Stopping %s", target)
	return c.service.Down(ctx, coreInstanceProjectName(target), opts)
}

// RemoveCoreInstanceDir removes the working dir of core network coreID
// (when the core network is deleted).
func (c *composeContext) RemoveCoreInstanceDir(coreID string) {
	c.materializeMu.Lock()
	defer c.materializeMu.Unlock()
	if err := os.RemoveAll(c.coreInstanceDir(CoreInstanceTarget(coreID))); err != nil {
		c.CtxLog.Errorf("Failed to remove working dir of %s: %v", CoreInstanceTarget(coreID), err)
	}
}

// coreCnRanNetworks are the cn-ran networks of the free5gc cores among
// docker network names, to remove on shutdown.
func coreCnRanNetworks(names []string) []sharedNetwork {
	out := make([]sharedNetwork, 0)
	seen := map[string]bool{}
	for _, name := range names {
		m := coreCnRanName.FindStringSubmatch(name)
		if m == nil || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, coreCnRanPool("", int(m[1][0]-'0')).network())
	}
	return out
}

// GnbCnRan is the cn-ran pool a lab-type gNB joins (the lab's, or a
// free5gc core's) and the AMF / UPF it dials there (the pool's reserved
// addresses).
type GnbCnRan struct {
	// the core network; "" for the lab free5GC
	CoreNetwork  string
	Network      string
	Prefix       netip.Prefix
	AmfIp, UpfIp string
	pool         IpamPool
}

func gnbCnRanOf(p IpamPool) GnbCnRan {
	amf, _ := p.ReservedAddr(RoleAmf)
	upf, _ := p.ReservedAddr(RoleUpf)
	return GnbCnRan{CoreNetwork: p.CoreID, Network: p.Network, Prefix: p.Subnet, AmfIp: amf.String(), UpfIp: upf.String(), pool: p}
}

// LabCnRan is the lab free5GC's cn-ran pool.
func LabCnRan() GnbCnRan {
	return gnbCnRanOf(labCnRanPool())
}

// CoreCnRan is the cn-ran pool of free5gc core network coreID in slot
// slot.
func CoreCnRan(coreID string, slot int) GnbCnRan {
	return gnbCnRanOf(coreCnRanPool(coreID, slot))
}

// PoolID is the IPAM pool n is.
func (n GnbCnRan) PoolID() string {
	return n.pool.ID
}

// N2Preview is the address a config preview shows for a gNB on n that is
// not deployed: the first of the gnb-n2 range.
func (n GnbCnRan) N2Preview() string {
	r, _ := n.pool.Range(RangeGnbN2)
	return r.First.String()
}

// WebconsoleBasePort is the host port the lab free5GC's webconsole is
// published on (deploy.webconsole.port); core network slot n publishes
// its own on WebconsoleBasePort() + n.
func (c *composeContext) WebconsoleBasePort() int {
	return c.webconsolePort
}
