package internal

import (
	"backend/model"
	"net/http"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
)

func (b *backend) getAccountRoutes() util.Routes {
	return util.Routes{
		{
			Name:        "Login",
			Method:      http.MethodPost,
			Pattern:     "/login",
			HandlerFunc: withLogging("Login", b.AccLog, b.handleLogin),
		},
		{
			Name:        "Logout",
			Method:      http.MethodPost,
			Pattern:     "/logout",
			HandlerFunc: withLogging("Logout", b.AccLog, b.handleLogout),
		},
	}
}

func (b *backend) handleLogin(c *gin.Context) {
	var req model.RequestLogin
	if err := c.ShouldBindJSON(&req); err != nil {
		b.AccLog.Warnf("Invalid login request from %s: %v\n", c.ClientIP(), err)
		c.JSON(http.StatusBadRequest, model.ResponseLogin{
			Message: "Invalid request",
		})
		return
	}

	response, errDetail := b.Processor.Login(&req)
	if errDetail != nil {
		b.AccLog.Warnf("Login failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseLogin{
			Message: errDetail.Detail,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleLogout(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
