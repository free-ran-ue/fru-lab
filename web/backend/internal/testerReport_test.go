package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReportRendersARun(t *testing.T) {
	page, err := renderTesterReport([]byte(sampleReport), time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, want := range []string{
		"<title>basic · 20261002-150405", // profile and run
		"Stopped by the user",
		"PDU session", // the stage table
		`id="report-data"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	if dir := os.Getenv("FRU_REPORT_OUT"); dir != "" { // to look at it
		_ = os.WriteFile(filepath.Join(dir, "sample-report.html"), page, 0o644)
	}
}

// Text from the run (a profile name, a failure cause) cannot break out of
// the page or its embedded data.
func TestReportEscapesRunText(t *testing.T) {
	evil := `</script><script>alert(1)</script>`
	raw := strings.Replace(sampleReport, `"profileName":"basic"`, `"profileName":"`+strings.ReplaceAll(evil, `"`, `\"`)+`"`, 1)
	page, err := renderTesterReport([]byte(raw), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(page), "<script>alert(1)") {
		t.Fatal("run text reached the page unescaped")
	}
}

func TestReportSettingsKeepOrderAndHideKeys(t *testing.T) {
	groups := profileSettings([]byte(`{"name":"x","scale":{"gnbCount":2,"ueCount":10},"ue":{"key":"8baf473f2f8fd09487cccbd7097c6862","dnn":"internet"},"network":{"n2":{"amfIp":"10.0.1.3"}}}`))
	if len(groups) != 3 || groups[0].Title != "Scale" || groups[1].Title != "UE template" {
		t.Fatalf("groups = %+v", groups)
	}
	if got := groups[0].Items; got[0] != (reportSetting{"gnbCount", "2"}) || got[1] != (reportSetting{"ueCount", "10"}) {
		t.Errorf("scale = %+v", got)
	}
	if got := groups[1].Items[0].Value; got != "8baf…6862" {
		t.Errorf("key shown as %q", got)
	}
	if got := groups[2].Items[0]; got != (reportSetting{"n2.amfIp", "10.0.1.3"}) {
		t.Errorf("nested = %+v", got)
	}
}

// A run stored with its tester names it in the report; an older run has
// no Tester entry.
func TestReportNamesTheTester(t *testing.T) {
	withName, err := withTester([]byte(sampleReport), storedTester{ID: "x", Name: "lab-2"})
	if err != nil {
		t.Fatal(err)
	}
	page, err := renderTesterReport(withName, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "Tester <b>lab-2</b>") {
		t.Fatal("report does not name the tester")
	}
	page, err = renderTesterReport([]byte(sampleReport), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(page), "Tester <b>") {
		t.Fatal("an older run names a tester")
	}
}

// The registration rate: the average is a tile; the per-second curve is
// a chart, which an older run without the curve leaves out.
func TestReportShowsTheRegistrationRate(t *testing.T) {
	raw := strings.Replace(sampleReport, `"registration":{"expected":4,"accepted":4,`, `"registration":{"expected":4,"accepted":4,"totalTimeMs":2000,`, 1)
	withCurve := strings.Replace(raw, `"runId":"20261002-150405",`,
		`"runId":"20261002-150405","setup":[{"t":1,"regPerSec":3,"pduPerSec":1,"failPerSec":0},{"t":2,"regPerSec":1,"pduPerSec":3,"failPerSec":0}],`, 1)
	page, err := renderTesterReport([]byte(withCurve), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, want := range []string{"Registrations · average", "2.0/s", "peak 3.0/s", "over 0:00:02", `data-setup`} {
		if !strings.Contains(html, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	page, err = renderTesterReport([]byte(raw), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if html := string(page); strings.Contains(html, `data-setup>`) || !strings.Contains(html, "2.0/s") {
		t.Error("an older run without the curve should keep the average and drop the chart")
	}
}

// Every stored run renders, including one with a control-plane loop and
// a setup curve (each section of the template is executed).
func TestReportRendersEverySection(t *testing.T) {
	raw := strings.Replace(sampleReport, `"runId":"20261002-150405",`,
		`"runId":"20261002-150405","setup":[{"t":1,"regPerSec":3,"pduPerSec":1,"failPerSec":0},{"t":2,"regPerSec":1,"pduPerSec":3,"failPerSec":0}],`+
			`"cpLoop":{"ues":5,"state":"stopped","durationSec":10,"cycles":7,"registration":{"accepted":8},"deregistration":{"accepted":7},"cleanup":{},"series":[{"t":1,"regPerSec":1,"deregPerSec":1,"failPerSec":0}]},`, 1)
	page, err := renderTesterReport([]byte(raw), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`id="cploop"`, "0.8/s", `data-setup`} {
		if !strings.Contains(string(page), want) {
			t.Errorf("report lacks %q", want)
		}
	}
}
