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
		{Name: "TesterProfileList", Method: http.MethodGet, Pattern: "/tester/profiles",
			HandlerFunc: b.handleTesterProfileList},
		{Name: "TesterProfileCreate", Method: http.MethodPost, Pattern: "/tester/profiles",
			HandlerFunc: withLogging("TesterProfileCreate", b.TesterLog, b.handleTesterProfileCreate)},
		{Name: "TesterProfileGet", Method: http.MethodGet, Pattern: "/tester/profiles/:id",
			HandlerFunc: b.handleTesterProfileGet},
		{Name: "TesterProfileUpdate", Method: http.MethodPut, Pattern: "/tester/profiles/:id",
			HandlerFunc: withLogging("TesterProfileUpdate", b.TesterLog, b.handleTesterProfileUpdate)},
		{Name: "TesterProfileDelete", Method: http.MethodDelete, Pattern: "/tester/profiles/:id",
			HandlerFunc: withLogging("TesterProfileDelete", b.TesterLog, b.handleTesterProfileDelete)},
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
		{Name: "TesterNetworkInterfaces", Method: http.MethodGet, Pattern: "/tester/network/interfaces",
			HandlerFunc: b.handleTesterProxy},
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

func (b *backend) handleTesterProfileList(c *gin.Context) {
	list, errDetail := b.Processor.TesterProfileList()
	respond(c, http.StatusOK, list, errDetail)
}

func (b *backend) handleTesterProfileGet(c *gin.Context) {
	profile, errDetail := b.Processor.TesterProfileGet(c.Param("id"))
	respond(c, http.StatusOK, profile, errDetail)
}

func (b *backend) handleTesterProfileCreate(c *gin.Context) {
	raw, ok := readTesterBody(c)
	if !ok {
		return
	}
	profile, errDetail := b.Processor.TesterProfileCreate(raw)
	respond(c, http.StatusCreated, profile, errDetail)
}

func (b *backend) handleTesterProfileUpdate(c *gin.Context) {
	raw, ok := readTesterBody(c)
	if !ok {
		return
	}
	profile, errDetail := b.Processor.TesterProfileUpdate(c.Param("id"), raw)
	respond(c, http.StatusOK, profile, errDetail)
}

func (b *backend) handleTesterProfileDelete(c *gin.Context) {
	response, errDetail := b.Processor.TesterProfileDelete(c.Param("id"))
	respond(c, http.StatusOK, response, errDetail)
}

// respond writes body with status, or errDetail's message.
func respond(c *gin.Context, status int, body any, errDetail *model.ErrorDetail) {
	if errDetail != nil {
		c.JSON(errDetail.HttpStatus, model.ResponseTesterAction{Message: errDetail.Detail})
		return
	}
	c.JSON(status, body)
}

func readTesterBody(c *gin.Context) ([]byte, bool) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxTesterBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ResponseTesterAction{Message: "Failed to read request body"})
		return nil, false
	}
	return raw, true
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
