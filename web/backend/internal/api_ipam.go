package internal

import (
	"net/http"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
)

// IPAM: the address plan of every network fru-lab manages and who holds
// which address.
func (b *backend) getIpamRoutes() util.Routes {
	return util.Routes{
		{Name: "IpamView", Method: http.MethodGet, Pattern: "/ipam", HandlerFunc: withLogging("IpamView", b.DeployLog, b.handleIpamView)},
	}
}

func (b *backend) handleIpamView(c *gin.Context) {
	view, detail := b.Processor.IpamView(c.Request.Context())
	respond(c, view, detail)
}
