package dataplane

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tester/cpuset"
)

// threadsOn counts this process's threads allowed exactly the CPUs list.
func threadsOn(t *testing.T, list string) int {
	t.Helper()
	tasks, err := filepath.Glob("/proc/self/task/*/status")
	require.NoError(t, err)
	n := 0
	for _, f := range tasks {
		b, err := os.ReadFile(f)
		if err != nil {
			continue // the thread ended
		}
		for _, line := range strings.Split(string(b), "\n") {
			if v, ok := strings.CutPrefix(line, "Cpus_allowed_list:"); ok && strings.TrimSpace(v) == list {
				n++
			}
		}
	}
	return n
}

// With data plane CPUs, every sender and reader runs on its own thread
// kept on them, and those threads end with the engine.
func TestSendersAndReadersKeepToTheDataPlaneCpus(t *testing.T) {
	all := cpuset.Allowed()
	if len(all) < 2 {
		t.Skip("needs two CPUs")
	}
	dp := cpuset.Set{all[len(all)-1]}
	before := threadsOn(t, dp.String())
	e, _ := startEngineWith(t, 2, 0, func(c *Config) { c.Cpus, c.Senders, c.Receivers = dp, 1, 1 })
	// 2 gNB readers + 1 sink reader + 2 uplink senders (one per gNB) + 1 downlink sender
	require.Eventually(t, func() bool { return threadsOn(t, dp.String())-before >= 6 }, 2*time.Second, 20*time.Millisecond)
	e.Stop(50 * time.Millisecond)
	// Their threads end with them; Go parks the main thread forever
	// instead of ending it, should one of them have run there.
	require.Eventually(t, func() bool { return threadsOn(t, dp.String()) <= before+1 }, 2*time.Second, 20*time.Millisecond)

	// threads Go starts afterwards are not pinned
	got := make(chan cpuset.Set, 20)
	for range cap(got) {
		go func() {
			runtime.LockOSThread()
			s, err := cpuset.Current()
			require.NoError(t, err)
			got <- s
			time.Sleep(50 * time.Millisecond)
		}()
	}
	for range cap(got) {
		require.Equal(t, all, <-got)
	}
}
