package context

import (
	"backend/constant"
	"backend/logger"
	"backend/model"
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
	dockernetwork "github.com/moby/moby/api/types/network"
	dockerclient "github.com/moby/moby/client"
)

// composeTemplate is one materializable template: where it lives in the
// embedded fs.
type composeTemplate struct {
	templateFS  embed.FS
	templateDir string
}

// composeTarget describes one deployable docker compose stack: its default
// template and what compose project name to use when driving it through the
// Docker Engine API. Most targets only ever have one template; free5gc is
// the exception, with a named variant per entry in variants (see
// DeployUp/Up's templateVariant parameter).
type composeTarget struct {
	projectName string
	composeTemplate
	variants map[string]composeTemplate
}

// template resolves which template to materialize for a deploy: the named
// variant if the target has one by that name, otherwise the target's
// default. An unknown/empty variant silently falls back to the default
// rather than erroring, since free5gc's own
// Down/Status/Logs calls) never pass one at all.
func (t composeTarget) template(variant string) composeTemplate {
	if variant != "" {
		if v, ok := t.variants[variant]; ok {
			return v
		}
	}
	return t.composeTemplate
}

// sharedNetwork is how fru-lab creates a docker network that is not owned
// by any single target's lifecycle (e.g. cn-ran, shared between free5gc
// and free-ran-ue): an IPAM pool's network (see IpamPool.network).
type sharedNetwork struct {
	name    string
	subnet  string
	gateway string
	// the IPAM ip-range Docker hands addresses out of; "" for the subnet
	ipRange    string
	bridgeName string
}

type composeContextIE struct {
	WorkDir string
	// WebconsolePort is the host port free5GC's webconsole is published on.
	WebconsolePort int
	// DeployTimeout bounds one deploy; a deploy that fails or runs over is
	// taken down again.
	DeployTimeout time.Duration
	// Ipam is the address plan and leases every address comes from.
	Ipam *ipamContext

	*logger.BackendLogger
}

type composeContext struct {
	dockerCli command.Cli
	service   api.Compose

	workDir        string
	webconsolePort int
	deployTimeout  time.Duration
	targets        map[string]composeTarget
	// every address and network comes from the IPAM plan
	ipam *ipamContext

	// guards the materialize-then-load sequence in loadProject: Up() writes
	// the template's files to workDir while docker is bind-mounting them
	// into new containers, so a concurrent status/logs poll must never be
	// allowed to wipe and rewrite the same files mid-mount (see loadProject).
	materializeMu sync.Mutex

	// networksReady is set once the shared networks exist; without a
	// Docker daemon at startup, the first deploy creates them.
	networksMu    sync.Mutex
	networksReady bool

	// gnbDeployMu serializes profile gNB deploys: checking what a gNB may
	// use, leasing its addresses and bringing it up (see LockGnbDeploys).
	gnbDeployMu sync.Mutex

	// test hooks standing in for the Docker daemon's networks; nil uses
	// Docker
	inspectNetworkFn func(ctx context.Context, name string) (map[netip.Addr]string, error)
	createNetworkFn  func(ctx context.Context, n sharedNetwork) error

	*logger.BackendLogger
}

func newComposeContext(ie *composeContextIE) (*composeContext, error) {
	dockerCli, err := command.NewDockerCli()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker cli: %v", err)
	}
	if err := dockerCli.Initialize(cliflags.NewClientOptions()); err != nil {
		return nil, fmt.Errorf("failed to initialize docker cli: %v", err)
	}

	service, err := compose.NewComposeService(dockerCli)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose service: %v", err)
	}

	c := &composeContext{
		dockerCli: dockerCli,
		service:   service,

		workDir:        ie.WorkDir,
		webconsolePort: ie.WebconsolePort,
		deployTimeout:  ie.DeployTimeout,
		targets:        defaultTargets(),
		ipam:           ie.Ipam,

		BackendLogger: ie.BackendLogger,
	}
	c.ipam.inspect = c.NetworkAddresses
	c.ipam.dockerNetworks = c.dockerNetworkSubnets

	// fru-lab still starts when the Docker daemon is not reachable (or a
	// network cannot be created): the deploy pages then fail with the
	// error, and the networks are created on the next deploy.
	if err := c.networks(context.Background()); err != nil {
		c.CtxLog.Warnf("The shared networks are not ready, deploying free5GC/gNB/UE will fail until they are: %v", err)
	}

	return c, nil
}

// defaultTargets are the deployable compose stacks and their templates.
func defaultTargets() map[string]composeTarget {
	return map[string]composeTarget{
		constant.DEPLOY_TARGET_FREE5GC: {
			projectName: "frulab-free5gc",
			composeTemplate: composeTemplate{
				templateFS:  free5gcTemplateFS,
				templateDir: "templates/free5gc",
			},
			variants: map[string]composeTemplate{
				constant.FREE5GC_TEMPLATE_BASIC: {
					templateFS:  free5gcTemplateFS,
					templateDir: "templates/free5gc",
				},
				constant.FREE5GC_TEMPLATE_ULCL: {
					templateFS:  free5gcUlclTemplateFS,
					templateDir: "templates/free5gc-ulcl",
				},
			},
		},
	}
}

// networks creates the shared networks unless that was already done.
func (c *composeContext) networks(ctx context.Context) error {
	c.networksMu.Lock()
	defer c.networksMu.Unlock()
	if c.networksReady {
		return nil
	}
	if err := c.ensureSharedNetworks(ctx); err != nil {
		return err
	}
	c.networksReady = true
	return nil
}

// ensureSharedNetworks creates the networks of the plan fru-lab creates
// up front: the lab core's cn-ran and ran-ue.
func (c *composeContext) ensureSharedNetworks(ctx context.Context) error {
	for _, p := range sharedPools() {
		if err := c.ensureNetwork(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// ensureNetwork creates the docker network of pool p unless it exists,
// refusing when its subnet overlaps another docker network or a host
// network. A network that exists is left as it is: one created before the
// plan had IPAM ip-ranges (frulab-ran-ue without one, so Docker hands UEs
// any address of the subnet) keeps working, and only gets its ip-range
// when it is created again (after fru-lab removed it on shutdown).
func (c *composeContext) ensureNetwork(ctx context.Context, p IpamPool) error {
	n := p.network()
	if c.createNetworkFn != nil {
		if err := c.ipam.checkOverlaps(ctx, p); err != nil {
			return err
		}
		return c.createNetworkFn(ctx, n)
	}
	apiClient := c.dockerCli.Client()
	if _, err := apiClient.NetworkInspect(ctx, n.name, dockerclient.NetworkInspectOptions{}); err == nil {
		c.CtxLog.Debugf("Shared network %s already exists", n.name)
		return nil
	}
	if err := c.ipam.checkOverlaps(ctx, p); err != nil {
		return err
	}

	ipamConfig, err := n.ipamConfig()
	if err != nil {
		return err
	}
	c.CtxLog.Infof("Creating shared network %s (%s, gateway %s, ip-range %q)", n.name, n.subnet, n.gateway, n.ipRange)
	if _, err := apiClient.NetworkCreate(ctx, n.name, dockerclient.NetworkCreateOptions{
		Driver: "bridge",
		IPAM: &dockernetwork.IPAM{
			Config: []dockernetwork.IPAMConfig{ipamConfig},
		},
		Options: map[string]string{
			"com.docker.network.bridge.name": n.bridgeName,
		},
	}); err != nil {
		return fmt.Errorf("failed to create network %s: %v", n.name, err)
	}
	return nil
}

// ipamConfig is the Docker IPAM config n is created with.
func (n sharedNetwork) ipamConfig() (dockernetwork.IPAMConfig, error) {
	var cfg dockernetwork.IPAMConfig
	var err error
	if cfg.Subnet, err = netip.ParsePrefix(n.subnet); err != nil {
		return cfg, fmt.Errorf("invalid subnet %s for network %s: %v", n.subnet, n.name, err)
	}
	if n.gateway != "" {
		if cfg.Gateway, err = netip.ParseAddr(n.gateway); err != nil {
			return cfg, fmt.Errorf("invalid gateway %s for network %s: %v", n.gateway, n.name, err)
		}
	}
	if n.ipRange != "" {
		if cfg.IPRange, err = netip.ParsePrefix(n.ipRange); err != nil {
			return cfg, fmt.Errorf("invalid ip-range %s for network %s: %v", n.ipRange, n.name, err)
		}
	}
	return cfg, nil
}

// dockerNetworkSubnets are the IPv4 subnets of every docker network by
// name.
func (c *composeContext) dockerNetworkSubnets(ctx context.Context) (map[string][]netip.Prefix, error) {
	list, err := c.dockerCli.Client().NetworkList(ctx, dockerclient.NetworkListOptions{})
	if err != nil {
		return nil, err
	}
	out := make(map[string][]netip.Prefix, len(list.Items))
	for _, item := range list.Items {
		for _, cfg := range item.IPAM.Config {
			if cfg.Subnet.IsValid() && cfg.Subnet.Addr().Is4() {
				out[item.Name] = append(out[item.Name], cfg.Subnet)
			}
		}
	}
	return out, nil
}

// loadProject parses a target's docker-compose.yaml into a *types.Project,
// materializing its built-in template onto disk first if forceMaterialize is
// set or nothing has been materialized yet (docker needs real files on disk
// for its bind-mounted volumes; the template itself may only ever live in
// the embedded binary).
//
// Every caller - Up, Down, Status, Logs - goes through this, so the
// materialize-then-read sequence is mutex-guarded: while Up() is deploying,
// docker is actively bind-mounting these exact files into new containers,
// and a status/logs poll landing mid-deploy must reuse what's already on
// disk rather than wipe and rewrite it out from under a running mount
// (os.CopyFS also refuses to write over files that already exist, so a
// re-materialize always needs a clean directory first).
//
// variant only matters when a (re)materialize actually happens: Down,
// Status and Logs always pass "" and get the target's default template
// whenever nothing is materialized yet (an edge case - e.g. polling status
// before ever deploying - where the choice is arbitrary since nothing is
// running). Only Up passes a real variant, since it's the only caller that
// forces a fresh materialize on every call.
func (c *composeContext) loadProject(ctx context.Context, target string, variant string, forceMaterialize bool) (*types.Project, error) {
	if IsGnbInstance(target) {
		return c.loadGnbInstanceProject(ctx, target)
	}
	if IsCoreInstance(target) {
		return c.loadCoreInstanceProject(ctx, target)
	}
	t, ok := c.targets[target]
	if !ok {
		return nil, fmt.Errorf("unsupported deploy target: %s", target)
	}

	workingDir := filepath.Join(c.workDir, target)

	c.materializeMu.Lock()
	defer c.materializeMu.Unlock()

	composeFilePath := filepath.Join(workingDir, "docker-compose.yaml")
	alreadyMaterialized := false
	if _, err := os.Stat(composeFilePath); err == nil {
		alreadyMaterialized = true
	}

	if forceMaterialize || !alreadyMaterialized {
		if err := os.RemoveAll(workingDir); err != nil {
			return nil, fmt.Errorf("failed to clean working dir %s: %v", workingDir, err)
		}
		if err := os.MkdirAll(workingDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create working dir %s: %v", workingDir, err)
		}

		tmpl := t.template(variant)
		templateFS, err := fs.Sub(tmpl.templateFS, tmpl.templateDir)
		if err != nil {
			return nil, fmt.Errorf("failed to open template for target %s: %v", target, err)
		}
		// the lab free5GC: the plan's lab cn-ran / SBI pools, no
		// container name prefix
		render := labCoreRender()
		if err := renderTemplateFS(templateFS, workingDir, render, render.funcs()); err != nil {
			return nil, fmt.Errorf("failed to materialize template for target %s: %v", target, err)
		}
		// go:embed drops the executable bit (embed.FS always reports a
		// fixed, non-executable mode), so anything a container runs as a
		// script needs it restored by hand after materializing.
		if err := restoreScriptPermissions(workingDir); err != nil {
			return nil, fmt.Errorf("failed to restore script permissions for target %s: %v", target, err)
		}
	}

	project, err := loader.LoadWithContext(ctx, types.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []types.ConfigFile{
			{Filename: filepath.Join(workingDir, "docker-compose.yaml")},
		},
	}, func(o *loader.Options) {
		o.SetProjectName(t.projectName, true)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load compose project for target %s: %v", target, err)
	}

	stampComposeLabels(project)
	if target == constant.DEPLOY_TARGET_FREE5GC {
		publishWebconsole(project, c.webconsolePort)
	}

	return project, nil
}

// webconsoleService is the free5GC webconsole's service name in every
// free5gc template; it listens on webconsoleContainerPort.
const (
	webconsoleService       = "webconsole"
	webconsoleContainerPort = 5000
)

// publishWebconsole publishes only the webconsole's HTTP port, on the host
// port fru-lab is configured with (deploy.webconsole.port), so a host that
// already uses 5000 can still deploy. Its FTP ports (2121/2122, CDR upload
// from the CHF) stay inside the compose network.
func publishWebconsole(project *types.Project, hostPort int) {
	service, ok := project.Services[webconsoleService]
	if !ok {
		return
	}
	service.Ports = []types.ServicePortConfig{{
		Mode: "ingress", Protocol: "tcp", Target: webconsoleContainerPort, Published: strconv.Itoa(hostPort),
	}}
	project.Services[webconsoleService] = service
}

// stampComposeLabels stamps the standard com.docker.compose.* labels onto
// every service in project - the loader alone doesn't do this (docker
// compose's own CLI does it itself after loading, in cmd/compose
// ProjectOptions.ToProject), so without it the compose engine can't find its
// own containers again once they're created.
func stampComposeLabels(project *types.Project) {
	for name, service := range project.Services {
		service.CustomLabels = map[string]string{
			api.ProjectLabel:     project.Name,
			api.ServiceLabel:     name,
			api.VersionLabel:     api.ComposeVersion,
			api.WorkingDirLabel:  project.WorkingDir,
			api.ConfigFilesLabel: strings.Join(project.ComposeFiles, ","),
			api.OneoffLabel:      "False",
		}
		project.Services[name] = service
	}
}

// restoreScriptPermissions makes every *.sh file under dir executable.
func restoreScriptPermissions(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".sh" {
			return nil
		}
		return os.Chmod(path, 0755)
	})
}

func (c *composeContext) Up(ctx context.Context, target string, templateVariant string) error {
	if target == constant.DEPLOY_TARGET_FREE5GC {
		// its compose project creates its SBI network
		if err := c.ipam.checkOverlaps(ctx, labSbiPool()); err != nil {
			return err
		}
	}
	project, err := c.loadProject(ctx, target, templateVariant, true)
	if err != nil {
		return err
	}

	c.CtxLog.Infof("Deploying target %s (project %s, template %s)", target, project.Name, templateVariant)
	return c.up(ctx, project)
}

// downAfterFailedUpTimeout bounds the cleanup after a failed deploy.
const downAfterFailedUpTimeout = time.Minute

// up starts project, giving up after deployTimeout. A deploy that fails
// or runs over is taken down again, so no container is left in "created"
// (which the dashboard would show as deploying forever), and the error
// says why it failed.
func (c *composeContext) up(ctx context.Context, project *types.Project) error {
	upCtx, cancel := context.WithTimeout(ctx, c.deployTimeout)
	defer cancel()
	if err := c.networks(upCtx); err != nil {
		return fmt.Errorf("failed to ensure shared networks: %v", err)
	}
	err := c.service.Up(upCtx, project, api.UpOptions{
		Create: api.CreateOptions{},
		Start: api.StartOptions{
			Project: project,
		},
	})
	if err == nil {
		return nil
	}
	if errors.Is(upCtx.Err(), context.DeadlineExceeded) {
		err = fmt.Errorf("did not finish within %s", c.deployTimeout)
	}
	c.CtxLog.Warnf("Deploy of %s failed, taking it down: %v", project.Name, err)
	downCtx, cancelDown := context.WithTimeout(context.Background(), downAfterFailedUpTimeout)
	defer cancelDown()
	if downErr := c.service.Down(downCtx, project.Name, api.DownOptions{Project: project}); downErr != nil {
		c.CtxLog.Errorf("Failed to take %s down after the failed deploy: %v", project.Name, downErr)
	}
	return explainUpError(err, c.publishedWebconsolePort(project))
}

// publishedWebconsolePort is the host port project publishes its
// webconsole on; the configured one when it has none.
func (c *composeContext) publishedWebconsolePort(project *types.Project) int {
	if svc, ok := project.Services[webconsoleService]; ok && len(svc.Ports) > 0 {
		if port, err := strconv.Atoi(svc.Ports[0].Published); err == nil {
			return port
		}
	}
	return c.webconsolePort
}

// explainUpError adds what to do about the usual cause of a failed deploy:
// a host port that is already taken.
func explainUpError(err error, webconsolePort int) error {
	msg := err.Error()
	if strings.Contains(msg, "port is already allocated") || strings.Contains(msg, "address already in use") {
		return fmt.Errorf("%w (a port this deployment publishes is already in use on the host; if it is the webconsole's port %d, set deploy.webconsole.port in config.yaml)", err, webconsolePort)
	}
	return err
}

func (c *composeContext) Down(ctx context.Context, target string) error {
	if IsCoreInstance(target) {
		return c.downCoreInstance(ctx, target)
	}
	project, err := c.loadProject(ctx, target, "", false)
	if err != nil {
		return err
	}

	c.CtxLog.Infof("Stopping target %s (project %s)", target, project.Name)
	return c.service.Down(ctx, project.Name, api.DownOptions{
		Project: project,
	})
}

type ComposeStatusResult struct {
	Services     []model.ServiceStatus
	LastDeployed *time.Time
}

func (c *composeContext) Status(ctx context.Context, target string) (*ComposeStatusResult, error) {
	// a profile gNB that is not deployed has no working dir; it is stopped
	if IsGnbInstance(target) && !c.gnbInstanceMaterialized(target) {
		return &ComposeStatusResult{Services: []model.ServiceStatus{}}, nil
	}
	// so is a free5gc core network that never was
	if IsCoreInstance(target) && !c.coreInstanceMaterialized(target) {
		return &ComposeStatusResult{Services: []model.ServiceStatus{}}, nil
	}
	project, err := c.loadProject(ctx, target, "", false)
	if err != nil {
		return nil, err
	}

	summaries, err := c.service.Ps(ctx, project.Name, api.PsOptions{Project: project, All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to get status for target %s: %v", target, err)
	}

	return summariesToStatusResult(summaries), nil
}

// summariesToStatusResult maps a raw `docker compose ps` result into our own
// status shape, shared by both the singleton targets (free5gc/gnb) and every
// per-instance ue deployment.
func summariesToStatusResult(summaries []api.ContainerSummary) *ComposeStatusResult {
	result := &ComposeStatusResult{
		Services: make([]model.ServiceStatus, 0, len(summaries)),
	}

	var lastCreated int64
	for _, summary := range summaries {
		result.Services = append(result.Services, model.ServiceStatus{
			Name:   summary.Service,
			Status: mapServiceStatus(string(summary.State), string(summary.Health)),
		})

		if summary.Created > lastCreated {
			lastCreated = summary.Created
		}
	}

	if lastCreated > 0 {
		deployedAt := time.Unix(lastCreated, 0)
		result.LastDeployed = &deployedAt
	}

	return result
}

const logTailLines = "200"

// services filters to specific compose services (e.g. just "amf") - nil or
// empty means every service in the project, matching the previous behavior.
func (c *composeContext) Logs(ctx context.Context, target string, services []string) ([]string, error) {
	project, err := c.loadProject(ctx, target, "", false)
	if err != nil {
		return nil, err
	}

	return c.fetchLogs(ctx, project, services)
}

// fetchLogs is shared by every singleton target and every ue instance.
func (c *composeContext) fetchLogs(ctx context.Context, project *types.Project, services []string) ([]string, error) {
	consumer := &lineLogConsumer{}
	if err := c.service.Logs(ctx, project.Name, consumer, api.LogOptions{
		Project:  project,
		Tail:     logTailLines,
		Services: services,
	}); err != nil {
		return nil, fmt.Errorf("failed to get logs for project %s: %v", project.Name, err)
	}

	return consumer.lines, nil
}

// ansiEscapeSequence matches ANSI SGR color/style codes (e.g. "\x1b[36m");
// many of the NFs colorize their own stdout, which renders as garbled
// control characters once copied verbatim into a plain <div>.
var ansiEscapeSequence = regexp.MustCompile("\x1b\\[[0-9;]*m")

// lineLogConsumer collects docker compose's per-container log lines into a
// single, flat slice for the API response; it doesn't stream.
type lineLogConsumer struct {
	lines []string
}

func (l *lineLogConsumer) Log(containerName, message string) {
	l.lines = append(l.lines, fmt.Sprintf("%-12s| %s", containerName, stripANSI(message)))
}

func (l *lineLogConsumer) Err(containerName, message string) {
	l.lines = append(l.lines, fmt.Sprintf("%-12s| [ERR] %s", containerName, stripANSI(message)))
}

func stripANSI(s string) string {
	return ansiEscapeSequence.ReplaceAllString(s, "")
}

func (l *lineLogConsumer) Status(_, _ string) {
	// container lifecycle status messages (e.g. "Attaching to ..."), not
	// real container output - nothing to collect
}

func mapServiceStatus(state, health string) string {
	switch {
	case health == "unhealthy":
		return "unhealthy"
	case state == "running":
		return "running"
	case state == "restarting", state == "created":
		return "deploying"
	default:
		return "stopped"
	}
}

func (c *composeContext) release() {
	c.CtxLog.Infoln("Release composeContext...")

	ctx, cancel := context.WithTimeout(context.Background(), teardownTimeout)
	defer cancel()
	c.teardown(ctx)

	if err := c.dockerCli.Client().Close(); err != nil {
		c.CtxLog.Errorf("Failed to close docker client: %v", err)
	}

	c.CtxLog.Infoln("composeContext released")
}

// teardownTimeout bounds the whole shutdown teardown: every UE, gNB and
// core container stopped, then the shared networks removed.
const teardownTimeout = 2 * time.Minute

// ueProjectPrefix starts every UE instance's compose project name (see
// ueProjectName).
const ueProjectPrefix = "frulab-ue-"

// teardown runs on shutdown and removes everything fru-lab created on the
// Docker host: the UE, gNB, free5gc core network and lab core compose
// projects (in that order, the one the backend enforces for single stops),
// then the shared networks, every core network's cn-ran network and their
// host bridge interfaces. A shared network cannot be removed while a
// container is still attached, so the projects go first.
func (c *composeContext) teardown(ctx context.Context) {
	projects, err := c.deployedProjects(ctx)
	if err != nil {
		c.CtxLog.Warnf("Docker is not reachable, nothing to tear down: %v", err)
		return
	}

	for _, name := range teardownOrder(projects, c.targets) {
		c.CtxLog.Infof("Tearing down project %s", name)
		if err := c.service.Down(ctx, name, api.DownOptions{RemoveOrphans: true}); err != nil {
			c.CtxLog.Errorf("Failed to tear down project %s: %v", name, err)
		}
	}

	c.removeSharedNetworks(ctx)
}

// deployedProjects lists the compose projects that have containers on the
// Docker host, running or not.
func (c *composeContext) deployedProjects(ctx context.Context) (map[string]bool, error) {
	list, err := c.dockerCli.Client().ContainerList(ctx, dockerclient.ContainerListOptions{
		All:     true,
		Filters: dockerclient.Filters{}.Add("label", api.ProjectLabel),
	})
	if err != nil {
		return nil, err
	}
	projects := map[string]bool{}
	for _, item := range list.Items {
		projects[item.Labels[api.ProjectLabel]] = true
	}
	return projects, nil
}

// legacyGnbProjects are the compose projects of the gNBs older fru-lab
// versions deployed (the singleton gNB and the Two Slice gNBs); they are
// still torn down if they are around.
var legacyGnbProjects = []string{"frulab-gnb-slice1", "frulab-gnb-slice2", "frulab-gnb"}

// teardownOrder picks fru-lab's own projects out of projects: UE instances
// first, then the profile gNBs, any legacy gNBs, then the free5gc core
// networks, then the lab core.
func teardownOrder(projects map[string]bool, targets map[string]composeTarget) []string {
	matching := func(match func(string) bool) []string {
		out := make([]string, 0)
		for name := range projects {
			if match(name) {
				out = append(out, name)
			}
		}
		sort.Strings(out)
		return out
	}
	var order []string
	order = append(order, matching(func(name string) bool { return strings.HasPrefix(name, ueProjectPrefix) })...)
	order = append(order, matching(gnbInstanceProject.MatchString)...)
	for _, name := range legacyGnbProjects {
		if projects[name] {
			order = append(order, name)
		}
	}
	order = append(order, matching(coreInstanceProject.MatchString)...)
	if name := targets[constant.DEPLOY_TARGET_FREE5GC].projectName; projects[name] {
		order = append(order, name)
	}
	return order
}

// removeSharedNetworks removes the networks of the plan fru-lab creates
// itself (the shared ones and the cn-ran network of every free5gc core
// network), plus any core cn-ran network left from a core network deleted
// meanwhile; docker deletes their bridge interfaces with them. A bridge
// interface that is somehow left on the host (it is only visible when
// fru-lab runs on the host, not in a container) is deleted too.
func (c *composeContext) removeSharedNetworks(ctx context.Context) {
	c.networksMu.Lock()
	defer c.networksMu.Unlock()

	apiClient := c.dockerCli.Client()
	networks := make([]sharedNetwork, 0)
	if plan, err := c.ipam.plan(); err != nil {
		c.CtxLog.Errorf("Failed to read the address plan, removing the shared networks only: %v", err)
		for _, p := range sharedPools() {
			networks = append(networks, p.network())
		}
	} else {
		for _, p := range plan {
			if p.managed {
				networks = append(networks, p.network())
			}
		}
	}
	list, err := apiClient.NetworkList(ctx, dockerclient.NetworkListOptions{
		Filters: dockerclient.Filters{}.Add("name", coreCnRanPrefix),
	})
	if err != nil {
		c.CtxLog.Errorf("Failed to list the core networks' cn-ran networks: %v", err)
	} else {
		names := make([]string, 0, len(list.Items))
		for _, item := range list.Items {
			names = append(names, item.Name)
		}
		networks = append(networks, coreCnRanNetworks(names)...)
	}

	seen := map[string]bool{}
	for _, n := range networks {
		if seen[n.name] {
			continue
		}
		seen[n.name] = true
		if _, err := apiClient.NetworkRemove(ctx, n.name, dockerclient.NetworkRemoveOptions{}); err != nil {
			if !cerrdefs.IsNotFound(err) {
				c.CtxLog.Errorf("Failed to remove network %s: %v", n.name, err)
			}
		} else {
			c.CtxLog.Infof("Removed network %s", n.name)
		}

		if _, err := net.InterfaceByName(n.bridgeName); err != nil {
			continue
		}
		c.CtxLog.Infof("Bridge %s is still on the host, deleting it", n.bridgeName)
		if out, err := exec.CommandContext(ctx, "ip", "link", "delete", n.bridgeName).CombinedOutput(); err != nil {
			c.CtxLog.Errorf("Failed to delete bridge %s: %v: %s", n.bridgeName, err, strings.TrimSpace(string(out)))
		}
	}
	c.networksReady = false
}

// UeInstanceConfig carries the subscriber-derived fields that get templated
// into one UE instance's uecfg.yaml at deploy time. RanIp is the one
// exception - it's not subscriber data, it's which gNB this UE should dial,
// picked by the processor layer (requested, or by the UE's Sd) since
// several gNBs can be up at once.
type UeInstanceConfig struct {
	Mcc          string
	Mnc          string
	Msin         string
	PermanentKey string
	OpValue      string
	Amf          string
	Sqn          string
	Dnn          string
	Sst          string
	Sd           string
	RanIp        string
	// the gNB target RanIp belongs to, recorded next to the instance
	Gnb string
}

// ue deployments are multi-instance - one per subscriber - unlike free5gc
// which is a process-wide singleton, so they can't reuse c.targets (keyed
// only by target name) or loadProject (which does a static, untemplated
// file copy). Every ue instance gets its own working directory and compose
// project name, keyed by instanceID (the subscriber's ueId).

func (c *composeContext) ueWorkingDir(instanceID string) string {
	return filepath.Join(c.workDir, constant.DEPLOY_TARGET_UE, instanceID)
}

func (c *composeContext) ueProjectName(instanceID string) string {
	return ueProjectPrefix + instanceID
}

// loadUeProject mirrors loadProject's materialize-then-load approach, but
// renders the ue template's *.tmpl files against cfg instead of copying them
// verbatim, since every instance needs its own container name/config. cfg is
// only required (and only used) when forceMaterialize actually needs to
// write fresh files; Down/Status/Logs pass nil and reuse whatever is already
// on disk, same reasoning as loadProject.
func (c *composeContext) loadUeProject(ctx context.Context, instanceID string, cfg *UeInstanceConfig, forceMaterialize bool) (*types.Project, error) {
	workingDir := c.ueWorkingDir(instanceID)

	c.materializeMu.Lock()
	defer c.materializeMu.Unlock()

	composeFilePath := filepath.Join(workingDir, "docker-compose.yaml")
	alreadyMaterialized := false
	if _, err := os.Stat(composeFilePath); err == nil {
		alreadyMaterialized = true
	}

	if forceMaterialize || !alreadyMaterialized {
		if cfg == nil {
			return nil, fmt.Errorf("ue instance %s has not been deployed yet", instanceID)
		}
		if err := os.RemoveAll(workingDir); err != nil {
			return nil, fmt.Errorf("failed to clean working dir %s: %v", workingDir, err)
		}
		if err := os.MkdirAll(workingDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create working dir %s: %v", workingDir, err)
		}

		templateFS, err := fs.Sub(ueTemplateFS, "templates/ue")
		if err != nil {
			return nil, fmt.Errorf("failed to open ue template: %v", err)
		}
		if err := renderTemplateFS(templateFS, workingDir, struct {
			InstanceID string
			*UeInstanceConfig
		}{InstanceID: instanceID, UeInstanceConfig: cfg}, nil); err != nil {
			return nil, fmt.Errorf("failed to materialize ue instance %s: %v", instanceID, err)
		}
		if err := restoreScriptPermissions(workingDir); err != nil {
			return nil, fmt.Errorf("failed to restore script permissions for ue instance %s: %v", instanceID, err)
		}
	}

	project, err := loader.LoadWithContext(ctx, types.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []types.ConfigFile{
			{Filename: filepath.Join(workingDir, "docker-compose.yaml")},
		},
	}, func(o *loader.Options) {
		o.SetProjectName(c.ueProjectName(instanceID), true)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load compose project for ue instance %s: %v", instanceID, err)
	}

	stampComposeLabels(project)

	return project, nil
}

// renderTemplateFS walks every file under src, rendering *.tmpl files as Go
// templates against data and funcs (writing the result without the .tmpl
// suffix; a missing key is an error) and copying every other file verbatim
// into dest.
func renderTemplateFS(src fs.FS, dest string, data any, funcs template.FuncMap) error {
	return fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		content, err := fs.ReadFile(src, path)
		if err != nil {
			return fmt.Errorf("failed to read template file %s: %v", path, err)
		}

		destPath := filepath.Join(dest, path)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".tmpl") {
			return os.WriteFile(destPath, content, 0644)
		}

		destPath = strings.TrimSuffix(destPath, ".tmpl")
		tmpl, err := template.New(filepath.Base(path)).Option("missingkey=error").Funcs(funcs).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template file %s: %v", path, err)
		}

		f, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %v", destPath, err)
		}
		defer func() { _ = f.Close() }()

		return tmpl.Execute(f, data)
	})
}

func (c *composeContext) UpUe(ctx context.Context, instanceID string, cfg *UeInstanceConfig) error {
	project, err := c.loadUeProject(ctx, instanceID, cfg, true)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(c.ueWorkingDir(instanceID), ueGnbFile), []byte(cfg.Gnb), 0644); err != nil {
		return err
	}

	c.CtxLog.Infof("Deploying ue instance %s on %s (project %s)", instanceID, cfg.Gnb, project.Name)
	return c.up(ctx, project)
}

func (c *composeContext) DownUe(ctx context.Context, instanceID string) error {
	project, err := c.loadUeProject(ctx, instanceID, nil, false)
	if err != nil {
		return err
	}

	c.CtxLog.Infof("Stopping ue instance %s (project %s)", instanceID, project.Name)
	return c.service.Down(ctx, project.Name, api.DownOptions{
		Project: project,
	})
}

func (c *composeContext) StatusUe(ctx context.Context, instanceID string) (*ComposeStatusResult, error) {
	project, err := c.loadUeProject(ctx, instanceID, nil, false)
	if err != nil {
		return nil, err
	}

	summaries, err := c.service.Ps(ctx, project.Name, api.PsOptions{Project: project, All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to get status for ue instance %s: %v", instanceID, err)
	}

	return summariesToStatusResult(summaries), nil
}

func (c *composeContext) LogsUe(ctx context.Context, instanceID string) ([]string, error) {
	project, err := c.loadUeProject(ctx, instanceID, nil, false)
	if err != nil {
		return nil, err
	}

	return c.fetchLogs(ctx, project, nil)
}

// ListUeInstances enumerates every ue instance that has ever been deployed
// (running or stopped) by reading the instance subdirectories materialized
// under <workDir>/ue - mirrors how a stopped free5gc/gnb still reports a
// "stopped" status rather than disappearing.
func (c *composeContext) ListUeInstances() ([]string, error) {
	dir := filepath.Join(c.workDir, constant.DEPLOY_TARGET_UE)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list ue instances: %v", err)
	}

	instances := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			instances = append(instances, entry.Name())
		}
	}

	return instances, nil
}

// ueGnbFile holds the gNB target a UE instance was deployed onto.
const ueGnbFile = "fru-lab-gnb"

// UeGnb is the gNB target a UE instance was last deployed onto; "" for one
// deployed before fru-lab recorded it.
func (c *composeContext) UeGnb(instanceID string) string {
	raw, err := os.ReadFile(filepath.Join(c.ueWorkingDir(instanceID), ueGnbFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func (c *composeContext) ueContainerName(instanceID string) string {
	return "ue-" + instanceID
}

// AttachUeShell execs an interactive shell inside a running ue instance's
// container and returns the raw, bidirectional hijacked stream plus the exec
// ID (needed for ResizeUeShell). TTY mode means stdout/stderr are already
// combined into one stream with no framing, so the caller can just copy
// bytes straight to/from a websocket without any stdcopy demuxing.
func (c *composeContext) AttachUeShell(ctx context.Context, instanceID string) (dockerclient.HijackedResponse, string, error) {
	return c.attachShell(ctx, c.ueContainerName(instanceID), "ue instance "+instanceID)
}

// AttachGnbShell is AttachUeShell for the gNB of profile profileID.
func (c *composeContext) AttachGnbShell(ctx context.Context, profileID string) (dockerclient.HijackedResponse, string, error) {
	target := GnbInstanceTarget(profileID)
	return c.attachShell(ctx, target, target)
}

// attachShell execs an interactive /bin/sh with a TTY in container.
func (c *composeContext) attachShell(ctx context.Context, container, what string) (dockerclient.HijackedResponse, string, error) {
	apiClient := c.dockerCli.Client()

	execCreated, err := apiClient.ExecCreate(ctx, container, dockerclient.ExecCreateOptions{
		Cmd:          []string{"/bin/sh"},
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		TTY:          true,
	})
	if err != nil {
		return dockerclient.HijackedResponse{}, "", fmt.Errorf("failed to create shell for %s: %v", what, err)
	}

	attached, err := apiClient.ExecAttach(ctx, execCreated.ID, dockerclient.ExecAttachOptions{TTY: true})
	if err != nil {
		return dockerclient.HijackedResponse{}, "", fmt.Errorf("failed to attach shell for %s: %v", what, err)
	}

	return attached.HijackedResponse, execCreated.ID, nil
}

// ResizeUeShell tells the container-side pty about the browser terminal's
// current size, so full-screen programs (vi, top, ...) render correctly.
func (c *composeContext) ResizeUeShell(ctx context.Context, execID string, height, width uint) error {
	_, err := c.dockerCli.Client().ExecResize(ctx, execID, dockerclient.ExecResizeOptions{
		Height: height,
		Width:  width,
	})
	return err
}
