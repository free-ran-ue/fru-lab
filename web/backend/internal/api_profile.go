package internal

import (
	"backend/model"
	"net/http"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
)

// gNB and UE profiles: saved configs the free-ran and free-ue pages deploy
// with, plus a render endpoint that previews the config file a profile
// produces.
func (b *backend) getProfileRoutes() util.Routes {
	return util.Routes{
		{Name: "GnbProfileList", Method: http.MethodGet, Pattern: "/profiles/gnb", HandlerFunc: withLogging("GnbProfileList", b.DeployLog, b.handleGnbProfileList)},
		{Name: "GnbProfileCreate", Method: http.MethodPost, Pattern: "/profiles/gnb", HandlerFunc: withLogging("GnbProfileCreate", b.DeployLog, b.handleGnbProfileSave)},
		{Name: "GnbProfileRender", Method: http.MethodPost, Pattern: "/profiles/gnb/render", HandlerFunc: withLogging("GnbProfileRender", b.DeployLog, b.handleGnbProfileRender)},
		{Name: "GnbProfileUpdate", Method: http.MethodPut, Pattern: "/profiles/gnb/:id", HandlerFunc: withLogging("GnbProfileUpdate", b.DeployLog, b.handleGnbProfileSave)},
		{Name: "GnbProfileDelete", Method: http.MethodDelete, Pattern: "/profiles/gnb/:id", HandlerFunc: withLogging("GnbProfileDelete", b.DeployLog, b.handleGnbProfileDelete)},
		{Name: "UeProfileList", Method: http.MethodGet, Pattern: "/profiles/ue", HandlerFunc: withLogging("UeProfileList", b.DeployLog, b.handleUeProfileList)},
		{Name: "UeProfileCreate", Method: http.MethodPost, Pattern: "/profiles/ue", HandlerFunc: withLogging("UeProfileCreate", b.DeployLog, b.handleUeProfileSave)},
		{Name: "UeProfileRender", Method: http.MethodPost, Pattern: "/profiles/ue/render", HandlerFunc: withLogging("UeProfileRender", b.DeployLog, b.handleUeProfileRender)},
		{Name: "UeProfileUpdate", Method: http.MethodPut, Pattern: "/profiles/ue/:id", HandlerFunc: withLogging("UeProfileUpdate", b.DeployLog, b.handleUeProfileSave)},
		{Name: "UeProfileDelete", Method: http.MethodDelete, Pattern: "/profiles/ue/:id", HandlerFunc: withLogging("UeProfileDelete", b.DeployLog, b.handleUeProfileDelete)},
	}
}

func respond[T any](c *gin.Context, v T, detail *model.ErrorDetail) {
	if detail != nil {
		c.JSON(detail.HttpStatus, model.ResponseDeployAction{Message: detail.Detail})
		return
	}
	c.JSON(http.StatusOK, v)
}

func bindOrBad(c *gin.Context, v any) bool {
	if err := c.ShouldBindJSON(v); err != nil {
		c.JSON(http.StatusBadRequest, model.ResponseDeployAction{Message: "Invalid request body: " + err.Error()})
		return false
	}
	return true
}

func (b *backend) handleGnbProfileList(c *gin.Context) {
	list, detail := b.Processor.ListGnbProfiles()
	respond(c, list, detail)
}

func (b *backend) handleGnbProfileSave(c *gin.Context) {
	var p model.GnbProfile
	if !bindOrBad(c, &p) {
		return
	}
	saved, detail := b.Processor.SaveGnbProfile(c.Param("id"), &p)
	respond(c, saved, detail)
}

func (b *backend) handleGnbProfileRender(c *gin.Context) {
	var p model.GnbProfile
	if !bindOrBad(c, &p) {
		return
	}
	out, detail := b.Processor.RenderGnbProfile(&p)
	respond(c, out, detail)
}

func (b *backend) handleGnbProfileDelete(c *gin.Context) {
	detail := b.Processor.DeleteGnbProfile(c.Param("id"))
	respond(c, model.ResponseDeployAction{Message: "Deleted"}, detail)
}

func (b *backend) handleUeProfileList(c *gin.Context) {
	list, detail := b.Processor.ListUeProfiles()
	respond(c, list, detail)
}

func (b *backend) handleUeProfileSave(c *gin.Context) {
	var p model.UeProfile
	if !bindOrBad(c, &p) {
		return
	}
	saved, detail := b.Processor.SaveUeProfile(c.Param("id"), &p)
	respond(c, saved, detail)
}

func (b *backend) handleUeProfileRender(c *gin.Context) {
	var p model.UeProfile
	if !bindOrBad(c, &p) {
		return
	}
	out, detail := b.Processor.RenderUeProfile(&p)
	respond(c, out, detail)
}

func (b *backend) handleUeProfileDelete(c *gin.Context) {
	detail := b.Processor.DeleteUeProfile(c.Param("id"))
	respond(c, model.ResponseDeployAction{Message: "Deleted"}, detail)
}
