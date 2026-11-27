package internal

import (
	"backend/model"
	"net/http"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
)

// Core networks: 5G cores besides the lab free5GC that gNBs can attach
// to - external ones, and other free5GCs fru-lab deploys on this host.
func (b *backend) getCoreNetworkRoutes() util.Routes {
	return util.Routes{
		{Name: "CoreNetworkList", Method: http.MethodGet, Pattern: "/core-networks", HandlerFunc: withLogging("CoreNetworkList", b.DeployLog, b.handleCoreNetworkList)},
		{Name: "CoreNetworkCreate", Method: http.MethodPost, Pattern: "/core-networks", HandlerFunc: withLogging("CoreNetworkCreate", b.DeployLog, b.handleCoreNetworkSave)},
		{Name: "CoreNetworkUpdate", Method: http.MethodPut, Pattern: "/core-networks/:id", HandlerFunc: withLogging("CoreNetworkUpdate", b.DeployLog, b.handleCoreNetworkSave)},
		{Name: "CoreNetworkDelete", Method: http.MethodDelete, Pattern: "/core-networks/:id", HandlerFunc: withLogging("CoreNetworkDelete", b.DeployLog, b.handleCoreNetworkDelete)},
	}
}

func (b *backend) handleCoreNetworkList(c *gin.Context) {
	list, detail := b.Processor.ListCoreNetworks()
	respond(c, list, detail)
}

func (b *backend) handleCoreNetworkSave(c *gin.Context) {
	var n model.CoreNetwork
	if !bindOrBad(c, &n) {
		return
	}
	saved, detail := b.Processor.SaveCoreNetwork(c.Param("id"), &n)
	respond(c, saved, detail)
}

func (b *backend) handleCoreNetworkDelete(c *gin.Context) {
	detail := b.Processor.DeleteCoreNetwork(c.Request.Context(), c.Param("id"))
	respond(c, model.ResponseDeployAction{Message: "Deleted"}, detail)
}

// handleCoreWebconsole proxies /api/core-networks/<id>/webconsole/<rest>
// to the webconsole of free5gc core network id, like /api/webconsole does
// for the lab free5GC.
func (b *backend) handleCoreWebconsole(c *gin.Context) {
	id := c.Param("id")
	port, detail := b.Processor.CoreWebconsolePort(id)
	if detail != nil {
		c.JSON(detail.HttpStatus, gin.H{"message": detail.Detail, "cause": detail.Detail})
		return
	}
	proxy, err := newWebconsoleProxy(b.webconsoleURLAt(port), "/api/core-networks/"+id+"/webconsole")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error(), "cause": err.Error()})
		return
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}
