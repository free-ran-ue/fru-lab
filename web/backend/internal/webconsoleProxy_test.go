package internal

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebconsoleProxyStripsThePrefixAndFruLabAuth(t *testing.T) {
	var gotPath, gotToken, gotAuth string
	webconsole := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotToken, gotAuth = r.URL.Path, r.Header.Get("Token"), r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"ueId":"imsi-208930000000001"}]`))
	}))
	defer webconsole.Close()

	proxy, err := newWebconsoleProxy(webconsole.URL)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/webconsole/api/subscriber", nil)
	req.Header.Set("Token", "webconsole-jwt")
	req.Header.Set("Authorization", "Bearer fru-lab-jwt")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	if rec.Code != http.StatusOK || string(body) != `[{"ueId":"imsi-208930000000001"}]` {
		t.Fatalf("got %d %s", rec.Code, body)
	}
	if gotPath != "/api/subscriber" {
		t.Fatalf("path %q; want the /api/webconsole prefix stripped", gotPath)
	}
	if gotToken != "webconsole-jwt" || gotAuth != "" {
		t.Fatalf("token %q auth %q; want the webconsole token kept and fru-lab's JWT dropped", gotToken, gotAuth)
	}
}

func TestWebconsoleProxyReportsAnUnreachableWebconsole(t *testing.T) {
	proxy, err := newWebconsoleProxy("http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/webconsole/api/subscriber", nil))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "webconsole is unreachable") || !strings.Contains(body, `"cause"`) {
		t.Fatalf("body %s; want a webconsole-style {cause} the subscriber pages show", body)
	}
}
