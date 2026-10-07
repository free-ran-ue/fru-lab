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
