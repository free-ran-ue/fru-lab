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
		{
			Name:        "DeployGnbUp",
			Method:      http.MethodPost,
			Pattern:     "/deploy/gnb",
			HandlerFunc: withLogging("DeployGnbUp", b.DeployLog, b.handleDeployGnbUp),
		},
		{
			Name:        "DeployGnbDown",
			Method:      http.MethodDelete,
			Pattern:     "/deploy/gnb",
			HandlerFunc: withLogging("DeployGnbDown", b.DeployLog, b.handleDeployGnbDown),
		},
		{
			Name:        "DeployGnbStatus",
			Method:      http.MethodGet,
			Pattern:     "/deploy/gnb/status",
			HandlerFunc: withLogging("DeployGnbStatus", b.DeployLog, b.handleDeployGnbStatus),
		},
		{
			Name:        "DeployGnbLogs",
			Method:      http.MethodGet,
			Pattern:     "/deploy/gnb/logs",
			HandlerFunc: withLogging("DeployGnbLogs", b.DeployLog, b.handleDeployGnbLogs),
		},
		{
			Name:        "DeployUeList",
			Method:      http.MethodGet,
			Pattern:     "/deploy/ue",
			HandlerFunc: withLogging("DeployUeList", b.DeployLog, b.handleDeployUeList),
		},
		{
			Name:        "DeployUeUp",
			Method:      http.MethodPost,
			Pattern:     "/deploy/ue/:instance",
			HandlerFunc: withLogging("DeployUeUp", b.DeployLog, b.handleDeployUeUp),
		},
		{
			Name:        "DeployUeDown",
			Method:      http.MethodDelete,
			Pattern:     "/deploy/ue/:instance",
			HandlerFunc: withLogging("DeployUeDown", b.DeployLog, b.handleDeployUeDown),
		},
		{
			Name:        "DeployUeStatus",
			Method:      http.MethodGet,
			Pattern:     "/deploy/ue/:instance/status",
			HandlerFunc: withLogging("DeployUeStatus", b.DeployLog, b.handleDeployUeStatus),
		},
		{
			Name:        "DeployUeLogs",
			Method:      http.MethodGet,
			Pattern:     "/deploy/ue/:instance/logs",
			HandlerFunc: withLogging("DeployUeLogs", b.DeployLog, b.handleDeployUeLogs),
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

func (b *backend) handleDeployGnbUp(c *gin.Context) {
	response, errDetail := b.Processor.DeployUp(c.Request.Context(), constant.DEPLOY_TARGET_GNB)
	if errDetail != nil {
		b.DeployLog.Warnf("Deploy up failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseDeployAction{
			Message: errDetail.Detail,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleDeployGnbDown(c *gin.Context) {
	response, errDetail := b.Processor.DeployDown(c.Request.Context(), constant.DEPLOY_TARGET_GNB)
	if errDetail != nil {
		b.DeployLog.Warnf("Deploy down failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseDeployAction{
			Message: errDetail.Detail,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleDeployGnbLogs(c *gin.Context) {
	response, errDetail := b.Processor.DeployLogs(c.Request.Context(), constant.DEPLOY_TARGET_GNB)
	if errDetail != nil {
		b.DeployLog.Warnf("Deploy logs failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseDeployLogs{
			Target: constant.DEPLOY_TARGET_GNB,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleDeployGnbStatus(c *gin.Context) {
	response, errDetail := b.Processor.DeployStatus(c.Request.Context(), constant.DEPLOY_TARGET_GNB)
	if errDetail != nil {
		b.DeployLog.Warnf("Deploy status failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseDeployStatus{
			Target: constant.DEPLOY_TARGET_GNB,
			Status: "unhealthy",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleDeployUeUp(c *gin.Context) {
	instance := c.Param("instance")

	var req model.RequestDeployUe
	if err := c.ShouldBindJSON(&req); err != nil {
		b.DeployLog.Warnf("Deploy ue up failed for %s: invalid request body: %v", c.ClientIP(), err)
		c.JSON(http.StatusBadRequest, model.ResponseDeployAction{
			Message: "Invalid request body",
		})
		return
	}

	response, errDetail := b.Processor.DeployUeUp(c.Request.Context(), instance, &req)
	if errDetail != nil {
		b.DeployLog.Warnf("Deploy ue up failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseDeployAction{
			Message: errDetail.Detail,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleDeployUeDown(c *gin.Context) {
	instance := c.Param("instance")

	response, errDetail := b.Processor.DeployUeDown(c.Request.Context(), instance)
	if errDetail != nil {
		b.DeployLog.Warnf("Deploy ue down failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseDeployAction{
			Message: errDetail.Detail,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleDeployUeStatus(c *gin.Context) {
	instance := c.Param("instance")

	response, errDetail := b.Processor.DeployUeStatus(c.Request.Context(), instance)
	if errDetail != nil {
		b.DeployLog.Warnf("Deploy ue status failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseDeployUeStatus{
			Instance: instance,
			Status:   "unhealthy",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleDeployUeLogs(c *gin.Context) {
	instance := c.Param("instance")

	response, errDetail := b.Processor.DeployUeLogs(c.Request.Context(), instance)
	if errDetail != nil {
		b.DeployLog.Warnf("Deploy ue logs failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseDeployUeLogs{
			Instance: instance,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleDeployUeList(c *gin.Context) {
	response, errDetail := b.Processor.DeployUeList(c.Request.Context())
	if errDetail != nil {
		b.DeployLog.Warnf("Deploy ue list failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseDeployUeList{})
		return
	}

	c.JSON(http.StatusOK, response)
}
