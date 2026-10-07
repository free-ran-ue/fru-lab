package internal

import (
	"backend/model"
	"io"
	"net/http"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
)

// getTesterRoutes are the Throughput Tester routes behind the JWT header
// middleware. Everything except the saved profile is forwarded to
// fru-tester unchanged (see newTesterProxy).
func (b *backend) getTesterRoutes() util.Routes {
	return util.Routes{
		{Name: "TesterProfileGet", Method: http.MethodGet, Pattern: "/tester/profile",
			HandlerFunc: withLogging("TesterProfileGet", b.TesterLog, b.handleTesterProfileGet)},
		{Name: "TesterProfilePut", Method: http.MethodPut, Pattern: "/tester/profile",
			HandlerFunc: withLogging("TesterProfilePut", b.TesterLog, b.handleTesterProfilePut)},
		{Name: "TesterProfileValidate", Method: http.MethodPost, Pattern: "/tester/profile/validate",
			HandlerFunc: withLogging("TesterProfileValidate", b.TesterLog, b.handleTesterProxy)},
		{Name: "TesterRunGet", Method: http.MethodGet, Pattern: "/tester/run",
			HandlerFunc: b.handleTesterProxy}, // polled; not logged per request
		{Name: "TesterRunStart", Method: http.MethodPost, Pattern: "/tester/run",
			HandlerFunc: withLogging("TesterRunStart", b.TesterLog, b.handleTesterProxy)},
		{Name: "TesterRunStop", Method: http.MethodPost, Pattern: "/tester/run/stop",
			HandlerFunc: withLogging("TesterRunStop", b.TesterLog, b.handleTesterProxy)},
		{Name: "TesterBenchGet", Method: http.MethodGet, Pattern: "/tester/bench",
			HandlerFunc: b.handleTesterProxy}, // polled; not logged per request
		{Name: "TesterBenchStart", Method: http.MethodPost, Pattern: "/tester/bench",
			HandlerFunc: withLogging("TesterBenchStart", b.TesterLog, b.handleTesterProxy)},
		{Name: "TesterNetworkPing", Method: http.MethodPost, Pattern: "/tester/network/ping",
			HandlerFunc: withLogging("TesterNetworkPing", b.TesterLog, b.handleTesterProxy)},
	}
}

// getTesterStreamRoutes: a browser WebSocket cannot send an Authorization
// header, so the stream checks ?token= itself, like the ue terminal.
func (b *backend) getTesterStreamRoutes() util.Routes {
	return util.Routes{
		{Name: "TesterRunStream", Method: http.MethodGet, Pattern: "/tester/run/stream",
			HandlerFunc: b.handleTesterStream},
	}
}

func (b *backend) handleTesterProfileGet(c *gin.Context) {
	raw, errDetail := b.Processor.TesterProfileGet()
	if errDetail != nil {
		c.JSON(errDetail.HttpStatus, model.ResponseTesterAction{Message: errDetail.Detail})
		return
	}
	if raw == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.Data(http.StatusOK, "application/json", raw)
}

func (b *backend) handleTesterProfilePut(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxTesterBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ResponseTesterAction{Message: "Failed to read request body"})
		return
	}
	response, errDetail := b.Processor.TesterProfilePut(raw)
	if errDetail != nil {
		c.JSON(errDetail.HttpStatus, model.ResponseTesterAction{Message: errDetail.Detail})
		return
	}
	c.JSON(http.StatusOK, response)
}

// maxTesterBodyBytes is one byte over the processor's 64 KiB cap, so an
// oversized body is reported as too large rather than cut and misparsed.
const maxTesterBodyBytes = 64<<10 + 1

func (b *backend) handleTesterProxy(c *gin.Context) {
	if b.testerProxy == nil {
		c.JSON(http.StatusServiceUnavailable, model.ResponseTesterAction{
			Message: "Throughput Tester is not configured: set backend.tester.url in config.yaml",
		})
		return
	}
	b.testerProxy.ServeHTTP(c.Writer, c.Request)
}

func (b *backend) handleTesterStream(c *gin.Context) {
	if _, err := util.ValidateJWT(c.Query("token"), b.jwt.secret); err != nil {
		c.JSON(http.StatusUnauthorized, model.ResponseTesterAction{Message: "Invalid token: " + err.Error()})
		return
	}
	b.handleTesterProxy(c)
}
