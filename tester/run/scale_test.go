package run

import (
	"encoding/json"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tester/profile"
)

// TestScale runs gNBs and UEs against the in-process fake AMF and reports
// memory, goroutines and the snapshot's size. Not a pass/fail test:
// FRU_TESTER_SCALE=<ues> go test ./run -run TestScale -v -timeout 2h
func TestScale(t *testing.T) {
	ues, _ := strconv.Atoi(os.Getenv("FRU_TESTER_SCALE"))
	if ues == 0 {
		t.Skip("set FRU_TESTER_SCALE=<UE count>")
	}
	gnbs := 10000
	if v, _ := strconv.Atoi(os.Getenv("FRU_TESTER_SCALE_GNBS")); v > 0 {
		gnbs = v
	}
	amf := newFakeAMF(nil)
	c, _ := newE2EController(amfDialer{amf: amf})
	p := e2eProfile()
	p.Scale = profile.Scale{GnbCount: gnbs, UeCount: ues}
	p.Network.N2.Cidr, p.Network.N2.StartIP = "10.0.0.0/8", "10.1.0.1"
	p.Network.N3.Cidr, p.Network.N3.StartIP = "11.0.0.0/8", "11.1.0.1"
	p.Network.N6.UePool = "10.0.0.0/8"
	fast := profile.ProcedureRate{RatePerSec: 100000, MaxInFlight: 2000, TimeoutMs: 30000, Retries: 0}
	p.Rates.Registration, p.Rates.Pdu, p.Rates.Deregistration = fast, fast, fast
	p.Rates.N2.TimeoutMs = 30000

	report := func(what string, start time.Time) {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		snapStart := time.Now()
		b, _ := json.Marshal(c.Snapshot())
		full := time.Since(snapStart)
		pageStart := time.Now()
		pb, _ := json.Marshal(c.SnapshotPage(GnbPage{Limit: 50}))
		t.Logf("%-22s t=%-8s heap=%5d MiB sys=%5d MiB goroutines=%-7d snapshot=%d KiB in %s, page of 50=%d KiB in %s",
			what, time.Since(start).Round(time.Second), m.HeapInuse>>20, m.Sys>>20, runtime.NumGoroutine(),
			len(b)>>10, full.Round(time.Millisecond), len(pb)>>10, time.Since(pageStart).Round(time.Millisecond))
	}
	start := time.Now()
	_, err := c.Start(p)
	require.NoError(t, err)
	report("started", start)
	deadline := time.Now().Add(2 * time.Hour)
	last := time.Now()
	for {
		s := c.Snapshot()
		if s.Error != "" {
			t.Fatalf("run failed: %s", s.Error)
		}
		if s.Pdu.Done && s.Registration.Done {
			break
		}
		if time.Since(last) > 30*time.Second {
			last = time.Now()
			t.Logf("  reg %d/%d pdu %d", s.Registration.Accepted, ues, s.Pdu.Accepted)
		}
		require.True(t, time.Now().Before(deadline))
		time.Sleep(200 * time.Millisecond)
	}
	report("all established", start)
	if path := os.Getenv("FRU_TESTER_SCALE_HEAP"); path != "" {
		f, err := os.Create(path)
		require.NoError(t, err)
		require.NoError(t, pprof.WriteHeapProfile(f))
		_ = f.Close()
	}
	s := c.Snapshot()
	t.Logf("reg accepted=%d failed=%d avg=%.1fms  pdu accepted=%d", s.Registration.Accepted,
		s.Registration.Rejected+s.Registration.TimedOut+s.Registration.Failed, s.Registration.AvgMs, s.Pdu.Accepted)
	stopStart := time.Now()
	_, err = c.Stop()
	require.NoError(t, err)
	for c.Snapshot().State != StateStopped {
		time.Sleep(200 * time.Millisecond)
	}
	report("stopped", stopStart)
}
