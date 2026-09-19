package internal

import (
	"backend/constant"
	"backend/model"
	"net/http"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
)

func (b *backend) getDeployRoutes() util.Routes {
	return util.Routes{
		{
			Name:        "DeployFree5gcUp",
			Method:      http.MethodPost,
			Pattern:     "/deploy/free5gc",
			HandlerFunc: withLogging("DeployFree5gcUp", b.DeployLog, b.handleDeployFree5gcUp),
		},
		{
			Name:        "DeployFree5gcDown",
			Method:      http.MethodDelete,
			Pattern:     "/deploy/free5gc",
			HandlerFunc: withLogging("DeployFree5gcDown", b.DeployLog, b.handleDeployFree5gcDown),
		},
		{
			Name:        "DeployFree5gcStatus",
			Method:      http.MethodGet,
			Pattern:     "/deploy/free5gc/status",
			HandlerFunc: withLogging("DeployFree5gcStatus", b.DeployLog, b.handleDeployFree5gcStatus),
		},
		{
			Name:        "DeployFree5gcLogs",
			Method:      http.MethodGet,
			Pattern:     "/deploy/free5gc/logs",
			HandlerFunc: withLogging("DeployFree5gcLogs", b.DeployLog, b.handleDeployFree5gcLogs),
		},
	}
}

func (b *backend) handleDeployFree5gcUp(c *gin.Context) {
	response, errDetail := b.Processor.DeployUp(c.Request.Context(), constant.DEPLOY_TARGET_FREE5GC)
	if errDetail != nil {
		b.DeployLog.Warnf("Deploy up failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseDeployAction{
			Message: errDetail.Detail,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleDeployFree5gcDown(c *gin.Context) {
	response, errDetail := b.Processor.DeployDown(c.Request.Context(), constant.DEPLOY_TARGET_FREE5GC)
	if errDetail != nil {
		b.DeployLog.Warnf("Deploy down failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseDeployAction{
			Message: errDetail.Detail,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleDeployFree5gcLogs(c *gin.Context) {
	response, errDetail := b.Processor.DeployLogs(c.Request.Context(), constant.DEPLOY_TARGET_FREE5GC)
	if errDetail != nil {
		b.DeployLog.Warnf("Deploy logs failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseDeployLogs{
			Target: constant.DEPLOY_TARGET_FREE5GC,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleDeployFree5gcStatus(c *gin.Context) {
	response, errDetail := b.Processor.DeployStatus(c.Request.Context(), constant.DEPLOY_TARGET_FREE5GC)
	if errDetail != nil {
		b.DeployLog.Warnf("Deploy status failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseDeployStatus{
			Target: constant.DEPLOY_TARGET_FREE5GC,
			Status: "unhealthy",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
