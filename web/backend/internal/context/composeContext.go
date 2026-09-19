package context

import (
	"backend/constant"
	"backend/logger"
	"backend/model"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"
	dockernetwork "github.com/docker/docker/api/types/network"
)

// composeTarget describes one deployable docker compose stack: where its
// built-in template lives (embedded fs) and what compose project name to
// use when driving it through the Docker Engine API.
type composeTarget struct {
	projectName string
	templateFS  embed.FS
	templateDir string
}

// sharedNetwork describes a docker network that is not owned by any single
// target's lifecycle (e.g. cn-ran, shared between free5gc and free-ran-ue).
type sharedNetwork struct {
	name       string
	subnet     string
	bridgeName string
}

type composeContextIE struct {
	WorkDir string

	*logger.BackendLogger
}

type composeContext struct {
	dockerCli command.Cli
	service   api.Compose

	workDir        string
	targets        map[string]composeTarget
	sharedNetworks []sharedNetwork

	// guards the materialize-then-load sequence in loadProject: Up() writes
	// the template's files to workDir while docker is bind-mounting them
	// into new containers, so a concurrent status/logs poll must never be
	// allowed to wipe and rewrite the same files mid-mount (see loadProject).
	materializeMu sync.Mutex

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

	c := &composeContext{
		dockerCli: dockerCli,
		service:   compose.NewComposeService(dockerCli),

		workDir: ie.WorkDir,
		targets: map[string]composeTarget{
			constant.DEPLOY_TARGET_FREE5GC: {
				projectName: "frulab-free5gc",
				templateFS:  free5gcTemplateFS,
				templateDir: "templates/free5gc",
			},
			constant.DEPLOY_TARGET_GNB: {
				projectName: "frulab-gnb",
				templateFS:  gnbTemplateFS,
				templateDir: "templates/gnb",
			},
		},
		sharedNetworks: []sharedNetwork{
			// free5gc (amf/upf) <-> gnb
			{name: "frulab-cn-ran", subnet: "10.0.1.0/24", bridgeName: "docker-cn-ran"},
			// gnb <-> ue (ue isn't a deploy target yet, but the network is
			// shared infrastructure independent of either target's lifecycle,
			// same reasoning as cn-ran, so it's created up front too)
			{name: "frulab-ran-ue", subnet: "10.0.2.0/24", bridgeName: "docker-ran-ue"},
		},

		BackendLogger: ie.BackendLogger,
	}

	if err := c.ensureSharedNetworks(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure shared networks: %v", err)
	}

	return c, nil
}

func (c *composeContext) ensureSharedNetworks(ctx context.Context) error {
	apiClient := c.dockerCli.Client()

	for _, n := range c.sharedNetworks {
		if _, err := apiClient.NetworkInspect(ctx, n.name, dockernetwork.InspectOptions{}); err == nil {
			c.CtxLog.Debugf("Shared network %s already exists", n.name)
			continue
		}

		c.CtxLog.Infof("Creating shared network %s (%s)", n.name, n.subnet)
		if _, err := apiClient.NetworkCreate(ctx, n.name, dockernetwork.CreateOptions{
			Driver: "bridge",
			IPAM: &dockernetwork.IPAM{
				Config: []dockernetwork.IPAMConfig{{Subnet: n.subnet}},
			},
			Options: map[string]string{
				"com.docker.network.bridge.name": n.bridgeName,
			},
		}); err != nil {
			return fmt.Errorf("failed to create network %s: %v", n.name, err)
		}
	}

	return nil
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
func (c *composeContext) loadProject(ctx context.Context, target string, forceMaterialize bool) (*types.Project, error) {
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

		templateFS, err := fs.Sub(t.templateFS, t.templateDir)
		if err != nil {
			return nil, fmt.Errorf("failed to open template for target %s: %v", target, err)
		}
		if err := os.CopyFS(workingDir, templateFS); err != nil {
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

	return project, nil
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

func (c *composeContext) Up(ctx context.Context, target string) error {
	project, err := c.loadProject(ctx, target, true)
	if err != nil {
		return err
	}

	c.CtxLog.Infof("Deploying target %s (project %s)", target, project.Name)
	return c.service.Up(ctx, project, api.UpOptions{
		Create: api.CreateOptions{},
		Start: api.StartOptions{
			Project: project,
		},
	})
}

func (c *composeContext) Down(ctx context.Context, target string) error {
	project, err := c.loadProject(ctx, target, false)
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
	project, err := c.loadProject(ctx, target, false)
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
			Status: mapServiceStatus(summary.State, summary.Health),
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

func (c *composeContext) Logs(ctx context.Context, target string) ([]string, error) {
	project, err := c.loadProject(ctx, target, false)
	if err != nil {
		return nil, err
	}

	return c.fetchLogs(ctx, project)
}

// fetchLogs is shared by every singleton target and every ue instance.
func (c *composeContext) fetchLogs(ctx context.Context, project *types.Project) ([]string, error) {
	consumer := &lineLogConsumer{}
	if err := c.service.Logs(ctx, project.Name, consumer, api.LogOptions{
		Project: project,
		Tail:    logTailLines,
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

	if err := c.dockerCli.Client().Close(); err != nil {
		c.CtxLog.Errorf("Failed to close docker client: %v", err)
	}

	c.CtxLog.Infoln("composeContext released")
}

// UeInstanceConfig carries the subscriber-derived fields that get templated
// into one UE instance's uecfg.yaml at deploy time. The gNB's own IP is
// fixed regardless of which subscriber this is (the UE always dials the one
// deployed gNB's static address), so it's never part of this - only the
// subscriber's own identity/auth material and slice/DNN selection vary.
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
}

// ue deployments are multi-instance - one per subscriber - unlike free5gc/gnb
// which are process-wide singletons, so they can't reuse c.targets (keyed
// only by target name) or loadProject (which does a static, untemplated
// file copy). Every ue instance gets its own working directory and compose
// project name, keyed by instanceID (the subscriber's ueId).

func (c *composeContext) ueWorkingDir(instanceID string) string {
	return filepath.Join(c.workDir, constant.DEPLOY_TARGET_UE, instanceID)
}

func (c *composeContext) ueProjectName(instanceID string) string {
	return "frulab-ue-" + instanceID
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
		}{InstanceID: instanceID, UeInstanceConfig: cfg}); err != nil {
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
// templates against data (writing the result without the .tmpl suffix) and
// copying every other file verbatim into dest.
func renderTemplateFS(src fs.FS, dest string, data any) error {
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
		tmpl, err := template.New(filepath.Base(path)).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template file %s: %v", path, err)
		}

		f, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %v", destPath, err)
		}
		defer f.Close()

		return tmpl.Execute(f, data)
	})
}

func (c *composeContext) UpUe(ctx context.Context, instanceID string, cfg *UeInstanceConfig) error {
	project, err := c.loadUeProject(ctx, instanceID, cfg, true)
	if err != nil {
		return err
	}

	c.CtxLog.Infof("Deploying ue instance %s (project %s)", instanceID, project.Name)
	return c.service.Up(ctx, project, api.UpOptions{
		Create: api.CreateOptions{},
		Start: api.StartOptions{
			Project: project,
		},
	})
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

	return c.fetchLogs(ctx, project)
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
