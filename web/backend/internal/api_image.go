package internal

import (
	"backend/model"
	"net/http"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
)

func (b *backend) getImageRoutes() util.Routes {
	return util.Routes{
		{
			Name:        "ImageList",
			Method:      http.MethodGet,
			Pattern:     "/images",
			HandlerFunc: withLogging("ImageList", b.DeployLog, b.handleImageList),
		},
		{
			Name:        "ImageRemove",
			Method:      http.MethodDelete,
			Pattern:     "/images/:key",
			HandlerFunc: withLogging("ImageRemove", b.DeployLog, b.handleImageRemove),
		},
		{
			Name:        "ImagePull",
			Method:      http.MethodPost,
			Pattern:     "/images/:key/pull",
			HandlerFunc: withLogging("ImagePull", b.DeployLog, b.handleImagePull),
		},
	}
}

func (b *backend) handleImageList(c *gin.Context) {
	response, errDetail := b.Processor.ImageList(c.Request.Context())
	if errDetail != nil {
		b.DeployLog.Warnf("Image list failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseImageList{})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleImageRemove(c *gin.Context) {
	key := c.Param("key")

	response, errDetail := b.Processor.ImageRemove(c.Request.Context(), key)
	if errDetail != nil {
		b.DeployLog.Warnf("Image remove failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseImageAction{
			Message: errDetail.Detail,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (b *backend) handleImagePull(c *gin.Context) {
	key := c.Param("key")

	response, errDetail := b.Processor.ImagePull(c.Request.Context(), key)
	if errDetail != nil {
		b.DeployLog.Warnf("Image pull failed for %s: %s", c.ClientIP(), errDetail.Detail)
		c.JSON(errDetail.HttpStatus, model.ResponseImageAction{
			Message: errDetail.Detail,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
