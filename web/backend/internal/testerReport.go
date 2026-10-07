package internal

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"
	"time"
)

// testerReportTemplate is the HTML report of one finished run: a single
// self-contained file (no external scripts, styles or fonts) that opens
// offline and prints. The report's JSON is embedded too, for the
// "raw data" download and the charts.
//
//go:embed templates/testerReport.html
var testerReportTemplate string

var testerReportTmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"bps":     formatBps,
	"bits":    formatBits,
	"ms":      formatMs,
	"loss":    formatLoss,
	"int":     formatInt,
	"pct":     formatPct,
	"secret":  maskSecret,
	"dash":    dash,
	"time":    func(t *time.Time) string { return formatTime(t) },
	"lossBad": func(r float64) bool { return r >= 0.001 },
	"rate":    func(v float64) string { return fmt.Sprintf("%.1f/s", v) },
	"okPct": func(s reportStage) string {
		n := s.Accepted + s.Rejected + s.TimedOut + s.Failed
		return formatPct(s.Accepted, n)
	},
}).Parse(testerReportTemplate))

type reportStage struct {
	Name        string  `json:"name"`
	Expected    int64   `json:"expected"`
	Attempted   int64   `json:"attempted"`
	Retries     int64   `json:"retries"`
	Accepted    int64   `json:"accepted"`
	Rejected    int64   `json:"rejected"`
	TimedOut    int64   `json:"timedOut"`
	Failed      int64   `json:"failed"`
	Skipped     int64   `json:"skipped"`
	TotalTimeMs float64 `json:"totalTimeMs"`
	AvgMs       float64 `json:"avgMs"`
	P50Ms       float64 `json:"p50Ms"`
	P95Ms       float64 `json:"p95Ms"`
	P99Ms       float64 `json:"p99Ms"`
	MaxMs       float64 `json:"maxMs"`
	Causes      []struct {
		Cause string `json:"cause"`
		Count int64  `json:"count"`
	} `json:"causes"`
	// Label is the stage's name on the page.
	Label string `json:"-"`
}

type reportDirection struct {
	TxPackets  uint64  `json:"txPackets"`
	TxBytes    uint64  `json:"txBytes"`
	RxPackets  uint64  `json:"rxPackets"`
	RxBytes    uint64  `json:"rxBytes"`
	LossRate   float64 `json:"lossRate"`
	OutOfOrder uint64  `json:"outOfOrder"`
	SendErrors uint64  `json:"sendErrors"`
	Misrouted  uint64  `json:"misrouted"`
	Latency    struct {
		Count uint64  `json:"count"`
		AvgMs float64 `json:"avgMs"`
		P50Ms float64 `json:"p50Ms"`
		P99Ms float64 `json:"p99Ms"`
		MaxMs float64 `json:"maxMs"`
	} `json:"latency"`
	// filled in by newReportView
	Label   string  `json:"-"`
	AvgBps  float64 `json:"-"` // received, over the traffic time
	PeakBps float64 `json:"-"` // highest received point of the chart
}

type reportGnb struct {
	Index       int     `json:"index"`
	Name        string  `json:"name"`
	GnbID       string  `json:"gnbId"`
	N2Ip        string  `json:"n2Ip"`
	N3Ip        string  `json:"n3Ip"`
	UeCount     int64   `json:"ueCount"`
	UeFirst     int     `json:"ueFirst"`
	UeLast      int     `json:"ueLast"`
	State       string  `json:"state"`
	LatencyMs   float64 `json:"latencyMs"`
	Cause       string  `json:"cause"`
	Registered  int64   `json:"registered"`
	Established int64   `json:"established"`
	// from the data plane's per-gNB counters
	UlTxBytes, UlRxBytes, DlTxBytes, DlRxBytes uint64 `json:"-"`
}

type reportPoint struct {
	T       float64 `json:"t"`
	UlTxBps float64 `json:"ulTxBps"`
	UlRxBps float64 `json:"ulRxBps"`
	DlTxBps float64 `json:"dlTxBps"`
	DlRxBps float64 `json:"dlRxBps"`
}

// reportCpLoop is the control-plane test (absent in older reports).
type reportCpLoop struct {
	Ues            int         `json:"ues"`
	State          string      `json:"state"`
	DurationSec    float64     `json:"durationSec"`
	Cycles         int64       `json:"cycles"`
	Registration   reportStage `json:"registration"`
	Deregistration reportStage `json:"deregistration"`
	Cleanup        reportStage `json:"cleanup"`
	Series         []struct {
		T, RegPerSec, DeregPerSec, FailPerSec float64
	} `json:"series"`
	// filled in by newReportView
	Procedures []reportStage `json:"-"`
	RegAvg     float64       `json:"-"` // accepted registrations per second
	DeregAvg   float64       `json:"-"`
	RegPeak    float64       `json:"-"`
	Duration   string        `json:"-"`
}

type reportSnapshot struct {
	RunID          string      `json:"runId"`
	ProfileName    string      `json:"profileName"`
	State          string      `json:"state"`
	Error          string      `json:"error"`
	StopReason     string      `json:"stopReason"`
	StartedAt      *time.Time  `json:"startedAt"`
	StoppedAt      *time.Time  `json:"stoppedAt"`
	N2             reportStage `json:"n2"`
	Registration   reportStage `json:"registration"`
	Pdu            reportStage `json:"pdu"`
	Deregistration reportStage `json:"deregistration"`
	N2Release      reportStage `json:"n2Release"`
	Gnbs           []reportGnb `json:"gnbs"`
	Ues            struct {
		Pending, Registering, Registered, Establishing, Established int64
		Failed, Skipped, Cancelled, Deregistering, Deregistered     int64
	} `json:"ues"`
	FailedUes []struct {
		Supi     string `json:"supi"`
		Gnb      string `json:"gnb"`
		Stage    string `json:"stage"`
		Cause    string `json:"cause"`
		Attempts int    `json:"attempts"`
	} `json:"failedUes"`
	Dataplane struct {
		Engine    string          `json:"engine"`
		ActiveUes int64           `json:"activeUes"`
		Ul        reportDirection `json:"ul"`
		Dl        reportDirection `json:"dl"`
		Gnbs      []struct {
			UlTxBytes uint64 `json:"ulTxBytes"`
			UlRxBytes uint64 `json:"ulRxBytes"`
			DlTxBytes uint64 `json:"dlTxBytes"`
			DlRxBytes uint64 `json:"dlRxBytes"`
		} `json:"gnbs"`
		Series []reportPoint `json:"series"`
	} `json:"dataplane"`
	CpLoop reportCpLoop `json:"cpLoop"`
}

// reportSetting is one line of the profile section.
type reportSetting struct{ Key, Value string }

type reportSettingGroup struct {
	Title string
	Items []reportSetting
}

// reportView is what the template renders.
type reportView struct {
	S           reportSnapshot
	Result      string
	ResultClass string // ok, bad or muted
	Duration    string
	TrafficTime string
	Stages      []reportStage
	Directions  []reportDirection
	UeStates    []reportSetting
	Settings    []reportSettingGroup
	Generated   string
	Raw         json.RawMessage
}

// renderTesterReport turns one stored run report into the HTML report.
func renderTesterReport(raw []byte, now time.Time) ([]byte, error) {
	var r struct {
		Profile  json.RawMessage `json:"profile"`
		Snapshot reportSnapshot  `json:"snapshot"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	v := newReportView(r.Snapshot, r.Profile, now)
	v.Raw = raw
	var b bytes.Buffer
	if err := testerReportTmpl.Execute(&b, v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func newReportView(s reportSnapshot, profile json.RawMessage, now time.Time) reportView {
	v := reportView{S: s, Generated: now.Local().Format("2006-01-02 15:04:05 MST")}
	switch {
	case s.State == "failed":
		v.Result, v.ResultClass = "Failed", "bad"
	case s.StopReason == "maxDuration":
		v.Result, v.ResultClass = "Stopped at the max run time", "ok"
	default:
		v.Result, v.ResultClass = "Stopped by the user", "ok"
	}
	if s.StartedAt != nil && s.StoppedAt != nil {
		v.Duration = formatDuration(s.StoppedAt.Sub(*s.StartedAt).Seconds())
	}

	for _, st := range []struct {
		stage *reportStage
		label string
	}{
		{&s.N2, "N2 setup"}, {&s.Registration, "Registration"}, {&s.Pdu, "PDU session"},
		{&s.Deregistration, map[bool]string{false: "UE deregistration", true: "Throughput UE deregistration"}[s.CpLoop.Ues > 0]},
		{&s.CpLoop.Cleanup, "Control-plane UE deregistration"}, {&s.N2Release, "gNB SCTP close"},
	} {
		if st.stage.Expected == 0 && st.stage.Attempted == 0 {
			continue // e.g. cleanup that never ran
		}
		st.stage.Label = st.label
		v.Stages = append(v.Stages, *st.stage)
	}

	// traffic time is the chart's last point: seconds since traffic began
	series := s.Dataplane.Series
	var trafficSec float64
	if n := len(series); n > 0 {
		trafficSec = series[n-1].T
		v.TrafficTime = formatDuration(trafficSec)
	}
	ul, dl := s.Dataplane.Ul, s.Dataplane.Dl
	ul.Label, dl.Label = "Uplink (gNB → UPF → N6)", "Downlink (N6 → UPF → gNB)"
	for _, p := range series {
		ul.PeakBps = math.Max(ul.PeakBps, p.UlRxBps)
		dl.PeakBps = math.Max(dl.PeakBps, p.DlRxBps)
	}
	if trafficSec > 0 {
		ul.AvgBps = float64(ul.RxBytes) * 8 / trafficSec
		dl.AvgBps = float64(dl.RxBytes) * 8 / trafficSec
	}
	v.Directions = []reportDirection{ul, dl}
	v.S.Dataplane.Ul, v.S.Dataplane.Dl = ul, dl

	for i := range v.S.Gnbs {
		if i < len(s.Dataplane.Gnbs) {
			t := s.Dataplane.Gnbs[i]
			g := &v.S.Gnbs[i]
			g.UlTxBytes, g.UlRxBytes, g.DlTxBytes, g.DlRxBytes = t.UlTxBytes, t.UlRxBytes, t.DlTxBytes, t.DlRxBytes
		}
	}

	u := s.Ues
	for _, kv := range []struct {
		k string
		n int64
	}{
		{"Deregistered", u.Deregistered}, {"Established", u.Established}, {"Registered", u.Registered},
		{"Failed", u.Failed}, {"Not started", u.Skipped}, {"Cancelled", u.Cancelled},
		{"Pending", u.Pending}, {"Registering", u.Registering}, {"Establishing", u.Establishing},
		{"Deregistering", u.Deregistering},
	} {
		if kv.n > 0 {
			v.UeStates = append(v.UeStates, reportSetting{kv.k, formatInt(kv.n)})
		}
	}

	if cp := &v.S.CpLoop; cp.Ues > 0 {
		cp.Registration.Label, cp.Deregistration.Label = "Registration", "Deregistration"
		cp.Procedures = []reportStage{cp.Registration, cp.Deregistration}
		if cp.DurationSec > 0 {
			cp.RegAvg = float64(cp.Registration.Accepted) / cp.DurationSec
			cp.DeregAvg = float64(cp.Deregistration.Accepted) / cp.DurationSec
			cp.Duration = formatDuration(cp.DurationSec)
		}
		for _, p := range cp.Series {
			cp.RegPeak = math.Max(cp.RegPeak, p.RegPerSec)
		}
	}

	v.Settings = profileSettings(profile)
	return v
}

// profileSettings flattens the profile into titled groups of key/value
// lines, in the profile's own field order.
func profileSettings(profile json.RawMessage) []reportSettingGroup {
	titles := map[string]string{
		"scale": "Scale", "gnb": "gNB template", "ue": "UE template", "network": "Network",
		"traffic": "Traffic per UE", "rates": "Pacing",
	}
	var groups []reportSettingGroup
	dec := json.NewDecoder(bytes.NewReader(profile))
	dec.UseNumber()
	var top orderedObject
	if err := dec.Decode(&top); err != nil {
		return nil
	}
	for _, f := range top {
		if f.key == "name" {
			continue
		}
		g := reportSettingGroup{Title: titles[f.key]}
		if g.Title == "" {
			g.Title = f.key
		}
		flatten(&g.Items, "", f.value)
		groups = append(groups, g)
	}
	return groups
}

type orderedField struct {
	key   string
	value any
}

// orderedObject is a JSON object that keeps its key order.
type orderedObject []orderedField

func (o *orderedObject) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if _, err := dec.Token(); err != nil { // {
		return err
	}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err
		}
		var val any
		if len(raw) > 0 && raw[0] == '{' {
			var sub orderedObject
			if err := sub.UnmarshalJSON(raw); err != nil {
				return err
			}
			val = sub
		} else {
			d := json.NewDecoder(bytes.NewReader(raw))
			d.UseNumber()
			if err := d.Decode(&val); err != nil {
				return err
			}
		}
		*o = append(*o, orderedField{t.(string), val})
	}
	return nil
}

func flatten(out *[]reportSetting, prefix string, v any) {
	switch x := v.(type) {
	case orderedObject:
		for _, f := range x {
			key := f.key
			if prefix != "" {
				key = prefix + "." + f.key
			}
			flatten(out, key, f.value)
		}
	default:
		s := fmt.Sprint(x)
		if x == nil || s == "" {
			s = "—"
		}
		if prefix == "key" || prefix == "opc" {
			s = maskSecret(s)
		}
		*out = append(*out, reportSetting{prefix, s})
	}
}

// dash is s, or a dash if it is empty.
func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// maskSecret keeps the first and last four characters of a key.
func maskSecret(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("•", len(s))
	}
	return s[:4] + "…" + s[len(s)-4:]
}

func formatBps(bps float64) string {
	if bps <= 0 {
		return "0 bps"
	}
	units := []string{"bps", "kbps", "Mbps", "Gbps", "Tbps"}
	i := 0
	for bps >= 1000 && i < len(units)-1 {
		bps /= 1000
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", bps, units[i])
	}
	return fmt.Sprintf("%.2f %s", bps, units[i])
}

// formatBits renders a byte count as bits, like the rates.
func formatBits(bytes uint64) string {
	v := float64(bytes) * 8
	units := []string{"b", "kb", "Mb", "Gb", "Tb"}
	i := 0
	for v >= 1000 && i < len(units)-1 {
		v /= 1000
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.2f %s", v, units[i])
}

func formatMs(ms float64) string {
	switch {
	case ms <= 0:
		return "—"
	case ms >= 1000:
		return fmt.Sprintf("%.2f s", ms/1000)
	case ms < 10:
		return fmt.Sprintf("%.2f ms", ms)
	}
	return fmt.Sprintf("%.0f ms", ms)
}

func formatLoss(rate float64) string {
	if rate > 0 && rate < 0.0001 {
		return fmt.Sprintf("%.4f %%", rate*100)
	}
	return fmt.Sprintf("%.2f %%", rate*100)
}

// formatPct is n of total as a percentage, or — without a total.
func formatPct(n, total int64) string {
	if total <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f %%", float64(n)*100/float64(total))
}

// formatInt groups thousands: 1,234,567.
func formatInt(n any) string {
	var v int64
	switch x := n.(type) {
	case int:
		v = int64(x)
	case int64:
		v = x
	case uint64:
		v = int64(x)
	}
	s := strconv.FormatInt(v, 10)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		return "-" + s
	}
	return s
}

func formatDuration(sec float64) string {
	s := int(math.Round(sec))
	return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.Local().Format("2006-01-02 15:04:05 MST")
}
