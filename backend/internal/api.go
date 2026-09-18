package internal

import (
	"net/http"

	loggergoModel "github.com/Alonza0314/logger-go/v2/model"
	"github.com/gin-gonic/gin"
)

// withLogging wraps a route's handler so the request is logged exactly once,
// after it completes, with the level derived from the actual response status
// code the handler wrote (via c.JSON/c.Status).
//
//	HandlerFunc: withLogging("Login", b.AccLog, b.handleLogin),
func withLogging(name string, lg loggergoModel.LoggerInterface, handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler(c)

		status := c.Writer.Status()
		switch {
		case status >= http.StatusInternalServerError:
			lg.Errorf("%s failed (status %d) for %s", name, status, c.ClientIP())
		case status >= http.StatusBadRequest:
			lg.Warnf("%s failed (status %d) for %s", name, status, c.ClientIP())
		default:
			lg.Infof("%s succeeded (status %d) for %s", name, status, c.ClientIP())
		}
	}
}
