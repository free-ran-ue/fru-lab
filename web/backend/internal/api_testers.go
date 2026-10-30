package internal

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"backend/model"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
)

// getTestersRoutes list, add and remove testers, and forward the Status
// and Setup pages' requests to one of them: /testers/<id>/<rest> reaches
// <tester>/api/<rest>. The other tester pages still use /tester/, the
// tester in config.yaml.
func (b *backend) getTestersRoutes() util.Routes {
	return util.Routes{
		{Name: "TestersList", Method: http.MethodGet, Pattern: "/testers",
			HandlerFunc: b.handleTestersList},
		{Name: "TestersAdd", Method: http.MethodPost, Pattern: "/testers",
			HandlerFunc: withLogging("TestersAdd", b.TesterLog, b.handleTestersAdd)},
		{Name: "TestersDelete", Method: http.MethodDelete, Pattern: "/testers/:id",
			HandlerFunc: withLogging("TestersDelete", b.TesterLog, b.handleTestersDelete)},
		{Name: "TesterByIdStatus", Method: http.MethodGet, Pattern: "/testers/:id/status",
			HandlerFunc: b.handleTesterByID}, // polled; not logged per request
		{Name: "TesterByIdRun", Method: http.MethodGet, Pattern: "/testers/:id/run",
			HandlerFunc: b.handleTesterByID},
		{Name: "TesterByIdRunStart", Method: http.MethodPost, Pattern: "/testers/:id/run",
			HandlerFunc: withLogging("TesterByIdRunStart", b.TesterLog, b.handleTesterByID)},
		{Name: "TesterByIdProfileValidate", Method: http.MethodPost, Pattern: "/testers/:id/profile/validate",
			HandlerFunc: b.handleTesterByID}, // on every edit; not logged per request
		{Name: "TesterByIdNetworkInterfaces", Method: http.MethodGet, Pattern: "/testers/:id/network/interfaces",
			HandlerFunc: b.handleTesterByID},
		{Name: "TesterByIdNetworkPing", Method: http.MethodPost, Pattern: "/testers/:id/network/ping",
			HandlerFunc: withLogging("TesterByIdNetworkPing", b.TesterLog, b.handleTesterByID)},
		{Name: "TesterByIdSystem", Method: http.MethodGet, Pattern: "/testers/:id/system",
			HandlerFunc: b.handleTesterByID},
		{Name: "TesterByIdCpuLoad", Method: http.MethodGet, Pattern: "/testers/:id/system/cpuload",
			HandlerFunc: b.handleTesterByID},
		{Name: "TesterByIdCpusPut", Method: http.MethodPut, Pattern: "/testers/:id/system/cpus",
			HandlerFunc: withLogging("TesterByIdCpusPut", b.TesterLog, b.handleTesterByID)},
	}
}

func (b *backend) handleTestersList(c *gin.Context) {
	c.JSON(http.StatusOK, b.testers.list())
}

func (b *backend) handleTestersAdd(c *gin.Context) {
	var req TesterAddRequest
	if err := json.NewDecoder(io.LimitReader(c.Request.Body, 64<<10)).Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ResponseTesterAction{Message: "Body must be {\"url\": ..., \"apiToken\": ...}: " + err.Error()})
		return
	}
	added, err := b.testers.add(c.Request.Context(), req.URL, req.ApiToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ResponseTesterAction{Message: err.Error()})
		return
	}
	b.TesterLog.Infof("Added tester %q at %s", added.Name, added.URL)
	c.JSON(http.StatusCreated, added)
}

func (b *backend) handleTestersDelete(c *gin.Context) {
	switch err := b.testers.remove(c.Param("id")); {
	case err == nil:
		c.JSON(http.StatusOK, model.ResponseTesterAction{Message: "Tester removed"})
	case errors.Is(err, errTesterNotFound):
		c.JSON(http.StatusNotFound, model.ResponseTesterAction{Message: err.Error()})
	case errors.Is(err, errTesterBuiltIn):
		c.JSON(http.StatusConflict, model.ResponseTesterAction{Message: err.Error()})
	default:
		b.TesterLog.Errorf("Failed to remove tester %s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, model.ResponseTesterAction{Message: "Failed to remove the tester"})
	}
}

func (b *backend) handleTesterByID(c *gin.Context) {
	t := b.testers.get(c.Param("id"))
	if t == nil {
		c.JSON(http.StatusNotFound, model.ResponseTesterAction{Message: "no such tester"})
		return
	}
	t.proxy.ServeHTTP(c.Writer, c.Request)
}
