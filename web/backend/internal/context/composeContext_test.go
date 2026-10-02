package context

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v2/pkg/api"

	"backend/constant"
	"backend/logger"
)

// fakeCompose stands in for the docker compose engine: Up runs upFn,
// Down records the project it was asked to take down.
type fakeCompose struct {
	api.Compose
	upFn func(ctx context.Context, p *types.Project) error

	mu      sync.Mutex
	upped   *types.Project
	downs   []string
	downCtx error // the context error Down saw
}

func (f *fakeCompose) Up(ctx context.Context, p *types.Project, _ api.UpOptions) error {
	f.mu.Lock()
	f.upped = p
	f.mu.Unlock()
	if f.upFn != nil {
		return f.upFn(ctx, p)
	}
	return nil
}

func (f *fakeCompose) Down(ctx context.Context, name string, _ api.DownOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.downs = append(f.downs, name)
	f.downCtx = ctx.Err()
	return nil
}

func newTestCompose(t *testing.T, fake *fakeCompose) *composeContext {
	t.Helper()
	return &composeContext{
		service:        fake,
		workDir:        t.TempDir(),
		targets:        defaultTargets(),
		webconsolePort: 5055,
		deployTimeout:  time.Minute,
		BackendLogger:  logger.NewBackendLogger("error", "", true),
	}
}

func TestUpPublishesTheWebconsoleOnTheConfiguredPortOnly(t *testing.T) {
	for _, variant := range []string{constant.FREE5GC_TEMPLATE_BASIC, constant.FREE5GC_TEMPLATE_ULCL, constant.FREE5GC_TEMPLATE_ULCL_2SLICE} {
		fake := &fakeCompose{}
		c := newTestCompose(t, fake)
		if err := c.Up(context.Background(), constant.DEPLOY_TARGET_FREE5GC, variant); err != nil {
			t.Fatalf("%s: %v", variant, err)
		}
		webconsole, ok := fake.upped.Services["webconsole"]
		if !ok {
			t.Fatalf("%s: no webconsole service", variant)
		}
		if len(webconsole.Ports) != 1 || webconsole.Ports[0].Target != 5000 || webconsole.Ports[0].Published != "5055" {
			t.Fatalf("%s: webconsole ports %+v; want only 5055->5000 (FTP 2121/2122 stay inside the compose network)", variant, webconsole.Ports)
		}
	}
}

func TestUpFailureTakesTheStackDownAndExplainsAPortConflict(t *testing.T) {
	fake := &fakeCompose{upFn: func(context.Context, *types.Project) error {
		return errors.New("Error response from daemon: driver failed programming external connectivity on endpoint webconsole: Bind for 0.0.0.0:5055 failed: port is already allocated")
	}}
	c := newTestCompose(t, fake)
	err := c.Up(context.Background(), constant.DEPLOY_TARGET_FREE5GC, constant.FREE5GC_TEMPLATE_BASIC)
	if err == nil {
		t.Fatal("want an error")
	}
	if len(fake.downs) != 1 || fake.downs[0] != "frulab-free5gc" {
		t.Fatalf("downs %v; a failed deploy must not leave containers stuck in created", fake.downs)
	}
	for _, want := range []string{"port is already allocated", "deploy.webconsole.port"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
}

func TestUpTimeoutTakesTheStackDown(t *testing.T) {
	fake := &fakeCompose{upFn: func(ctx context.Context, _ *types.Project) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	c := newTestCompose(t, fake)
	c.deployTimeout = 50 * time.Millisecond
	start := time.Now()
	err := c.Up(context.Background(), constant.DEPLOY_TARGET_GNB, "")
	if err == nil || !strings.Contains(err.Error(), "did not finish within 50ms") {
		t.Fatalf("error %v; want a timeout", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("Up did not give up at the timeout")
	}
	if len(fake.downs) != 1 || fake.downs[0] != "frulab-gnb" {
		t.Fatalf("downs %v", fake.downs)
	}
	if fake.downCtx != nil {
		t.Fatalf("down ran with an expired context (%v); it must get its own", fake.downCtx)
	}
}

func TestUeUpFailureTakesTheInstanceDown(t *testing.T) {
	fake := &fakeCompose{upFn: func(context.Context, *types.Project) error { return errors.New("image pull failed") }}
	c := newTestCompose(t, fake)
	err := c.UpUe(context.Background(), "imsi-208930000000001", &UeInstanceConfig{
		Mcc: "208", Mnc: "93", Msin: "0000000001", PermanentKey: "k", OpValue: "o", Amf: "8000", Sqn: "0", Dnn: "internet", Sst: "1", Sd: "010203", RanIp: "10.0.2.2",
	})
	if err == nil || !strings.Contains(err.Error(), "image pull failed") {
		t.Fatalf("error %v", err)
	}
	if len(fake.downs) != 1 {
		t.Fatalf("downs %v", fake.downs)
	}
}
