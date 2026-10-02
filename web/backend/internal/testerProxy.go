package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// newTesterProxy forwards /api/tester/<rest> to <testerURL>/api/<rest>.
// The caller has already checked the user's JWT; the proxy replaces it
// with fru-tester's own API token and drops the ?token= query param the
// WebSocket route uses. WebSocket upgrades pass through unchanged.
func newTesterProxy(testerURL, apiToken string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(testerURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("tester.url %q must be an absolute http(s) URL", testerURL)
	}
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Scheme = target.Scheme
			pr.Out.URL.Host = target.Host
			pr.Out.URL.Path = "/api" + strings.TrimPrefix(pr.In.URL.Path, "/api/tester")
			pr.Out.URL.RawPath = ""
			q := pr.Out.URL.Query()
			q.Del("token")
			pr.Out.URL.RawQuery = q.Encode()
			pr.Out.Host = target.Host
			pr.Out.Header.Set("Authorization", "Bearer "+apiToken)
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"message": "fru-tester is unreachable: " + err.Error(),
			})
		},
	}, nil
}
