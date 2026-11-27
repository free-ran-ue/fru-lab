package internal

import (
	"backend/model"
	"net/http"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
)

// Core networks: 5G cores outside the lab (besides the lab free5GC) that
// gNBs can attach to.
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
	detail := b.Processor.DeleteCoreNetwork(c.Param("id"))
	respond(c, model.ResponseDeployAction{Message: "Deleted"}, detail)
}
