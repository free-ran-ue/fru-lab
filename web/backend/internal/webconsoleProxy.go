package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// newWebconsoleProxy forwards /api/webconsole/<rest> to <webconsoleURL>/<rest>,
// so the browser reaches free5GC's webconsole through fru-lab itself (same
// origin, whatever port the webconsole is published on). The caller has
// already checked the user's fru-lab JWT, which is dropped here; the
// webconsole's own "Token" header passes through.
func newWebconsoleProxy(webconsoleURL string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(webconsoleURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("webconsole url %q must be an absolute http(s) URL", webconsoleURL)
	}
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Scheme = target.Scheme
			pr.Out.URL.Host = target.Host
			pr.Out.URL.Path = strings.TrimPrefix(pr.In.URL.Path, "/api/webconsole")
			pr.Out.URL.RawPath = ""
			pr.Out.Host = target.Host
			pr.Out.Header.Del("Authorization")
		},
		// answer like the webconsole does ({"cause": ...}), which is what
		// the subscriber pages display
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"cause": fmt.Sprintf("free5GC webconsole is unreachable at %s (is free5GC deployed?): %v", webconsoleURL, err),
			})
		},
	}, nil
}
