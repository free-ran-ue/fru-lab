package internal

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestTesterProxyRewritesPathAndAuth(t *testing.T) {
	var gotPath, gotQuery, gotAuth, gotBody string
	tester := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotAuth = r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"state":"configuring"}`))
	}))
	defer tester.Close()

	proxy, err := newTesterProxy(tester.URL, "engine-token")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/tester/run?token=user-jwt&x=1", strings.NewReader(`{"name":"p"}`))
	req.Header.Set("Authorization", "Bearer user-jwt")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted || rec.Body.String() != `{"state":"configuring"}` {
		t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
	}
	if gotPath != "/api/run" || gotQuery != "x=1" || gotAuth != "Bearer engine-token" || gotBody != `{"name":"p"}` {
		t.Fatalf("tester saw path=%q query=%q auth=%q body=%q", gotPath, gotQuery, gotAuth, gotBody)
	}
}

func TestTesterProxyReportsUnreachableAs502(t *testing.T) {
	tester := httptest.NewServer(http.NotFoundHandler())
	url := tester.URL
	tester.Close() // nothing listens there now

	proxy, err := newTesterProxy(url, "t")
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tester/run", nil))
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusBadGateway || !strings.HasPrefix(body["message"], "fru-tester is unreachable") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestTesterProxyPassesWebSocket(t *testing.T) {
	up := websocket.Upgrader{}
	tester := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/run/stream" || r.Header.Get("Authorization") != "Bearer t" {
			http.Error(w, "bad", http.StatusUnauthorized)
			return
		}
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		_ = c.WriteMessage(websocket.TextMessage, []byte(`{"state":"n2"}`))
	}))
	defer tester.Close()

	proxy, err := newTesterProxy(tester.URL, "t")
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(proxy)
	defer front.Close()

	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(front.URL, "http")+"/api/tester/run/stream?token=jwt", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.Close() }()
	_, msg, err := ws.ReadMessage()
	if err != nil || string(msg) != `{"state":"n2"}` {
		t.Fatalf("got %q, %v", msg, err)
	}
}

func TestNewTesterProxyRejectsRelativeURL(t *testing.T) {
	if _, err := newTesterProxy("localhost:9100", "t"); err == nil {
		t.Fatal("want error for URL without scheme")
	}
}

func TestTesterRoutesAnswer503WhenNotConfigured(t *testing.T) {
	b := &backend{} // tester.url empty -> no proxy
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/tester/run", nil)
	b.handleTesterProxy(c)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "backend.tester.url") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}
