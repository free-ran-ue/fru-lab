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
		},
		sharedNetworks: []sharedNetwork{
			{name: "frulab-cn-ran", subnet: "10.0.1.0/24", bridgeName: "docker-cn-ran"},
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

	// the loader alone doesn't stamp the standard com.docker.compose.*
	// labels onto each service; docker compose's own CLI does this itself
	// after loading (cmd/compose ProjectOptions.ToProject), so we have to
	// do it too, or the compose engine can't find its own containers again
	// once they're created.
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

	return project, nil
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

	return result, nil
}

const logTailLines = "200"

func (c *composeContext) Logs(ctx context.Context, target string) ([]string, error) {
	project, err := c.loadProject(ctx, target, false)
	if err != nil {
		return nil, err
	}

	consumer := &lineLogConsumer{}
	if err := c.service.Logs(ctx, project.Name, consumer, api.LogOptions{
		Project: project,
		Tail:    logTailLines,
	}); err != nil {
		return nil, fmt.Errorf("failed to get logs for target %s: %v", target, err)
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
