package internal

import (
	"backend/logger"
	"testing"

	"github.com/gin-gonic/gin"
)

// Every route registers without a gin conflict (the core networks' CRUD
// routes next to their webconsole proxy).
func TestRoutesRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	addServices(router, &backend{BackendLogger: logger.NewBackendLogger("error", "", true)})
	found := map[string]bool{}
	for _, r := range router.Routes() {
		found[r.Method+" "+r.Path] = true
	}
	for _, want := range []string{
		"GET /api/deploy/cores", "POST /api/deploy/cores/:id", "DELETE /api/deploy/cores/:id", "GET /api/deploy/cores/:id/logs",
		"PUT /api/core-networks/:id", "GET /api/core-networks/:id/webconsole/*path", "POST /api/core-networks/:id/webconsole/*path",
		"GET /api/ipam",
	} {
		if !found[want] {
			t.Errorf("route %s is missing", want)
		}
	}
}
