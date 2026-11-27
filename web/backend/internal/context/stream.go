package context

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/compose/v5/pkg/api"
)

// LogLine is one line of a container's output.
type LogLine struct {
	Time   time.Time `json:"t"`
	Source string    `json:"src"`
	Line   string    `json:"line"`
	Stderr bool      `json:"err,omitempty"`
}

type funcLogConsumer struct {
	onLine func(LogLine)
}

// docker prefixes each line with an RFC 3339 timestamp when asked to.
func splitTimestamp(message string) (time.Time, string) {
	if i := strings.IndexByte(message, ' '); i > 0 {
		if t, err := time.Parse(time.RFC3339Nano, message[:i]); err == nil {
			return t, message[i+1:]
		}
	}
	return time.Now(), message
}

func (f *funcLogConsumer) emit(container, message string, stderr bool) {
	t, line := splitTimestamp(stripANSI(message))
	f.onLine(LogLine{Time: t, Source: container, Line: strings.TrimRight(line, "\r\n"), Stderr: stderr})
}

func (f *funcLogConsumer) Log(container, message string) { f.emit(container, message, false) }
func (f *funcLogConsumer) Err(container, message string) { f.emit(container, message, true) }
func (f *funcLogConsumer) Status(_, _ string)            {}

// FollowLogs streams a target's (or, for target "ue", one UE instance's)
// logs to onLine: the last `tail` lines, then every new one, until ctx
// ends.
func (c *composeContext) FollowLogs(ctx context.Context, target, instance string, services []string, tail int, onLine func(LogLine)) error {
	var name string
	var opts api.LogOptions
	if target == "ue" {
		project, err := c.loadUeProject(ctx, instance, nil, false)
		if err != nil {
			return err
		}
		name, opts.Project = project.Name, project
	} else {
		project, err := c.loadProject(ctx, target, "", false)
		if err != nil {
			return err
		}
		name, opts.Project = project.Name, project
	}
	opts.Services = services
	opts.Follow = true
	opts.Timestamps = true
	opts.Tail = fmt.Sprint(tail)
	return c.service.Logs(ctx, name, &funcLogConsumer{onLine: onLine}, opts)
}
