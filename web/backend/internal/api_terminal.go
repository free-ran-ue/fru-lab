package internal

import (
	"encoding/json"
	"net/http"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var terminalUpgrader = websocket.Upgrader{
	// matches the wide-open CORS policy the rest of this API already uses
	// (see addMiddleware) - this is a lab tool, not a multi-tenant service.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// resizeMessage is the one control message the frontend's terminal sends;
// anything else read off the socket is raw keystroke input for the shell.
type resizeMessage struct {
	Type string `json:"type"`
	Cols uint   `json:"cols"`
	Rows uint   `json:"rows"`
}

func (b *backend) getTerminalRoutes() util.Routes {
	return util.Routes{
		{
			Name:        "DeployUeTerminal",
			Method:      http.MethodGet,
			Pattern:     "/deploy/ue/:instance/terminal",
			HandlerFunc: b.handleUeTerminal,
		},
	}
}

func (b *backend) handleUeTerminal(c *gin.Context) {
	if _, err := util.ValidateJWT(c.Query("token"), b.jwt.secret); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid token: " + err.Error()})
		return
	}

	instance := c.Param("instance")

	conn, err := terminalUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		b.DeployLog.Warnf("Failed to upgrade websocket for ue terminal %s: %v", instance, err)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			b.DeployLog.Warnf("Failed to close websocket for ue terminal %s: %v", instance, err)
		}
	}()

	hijacked, execID, err := b.FlContext.AttachUeShell(c.Request.Context(), instance)
	if err != nil {
		b.DeployLog.Warnf("Failed to attach terminal for ue instance %s: %v", instance, err)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n["+err.Error()+"]\r\n"))
		return
	}
	defer hijacked.Close()

	b.DeployLog.Infof("Terminal attached for ue instance %s (exec %s)", instance, execID)

	done := make(chan struct{})

	// container -> browser
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := hijacked.Reader.Read(buf)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// browser -> container (keystrokes, or a {"type":"resize",...} control message)
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if messageType == websocket.TextMessage {
			var resize resizeMessage
			if err := json.Unmarshal(data, &resize); err == nil && resize.Type == "resize" {
				if err := b.FlContext.ResizeUeShell(c.Request.Context(), execID, resize.Rows, resize.Cols); err != nil {
					b.DeployLog.Warnf("Failed to resize terminal for ue instance %s: %v", instance, err)
				}
				continue
			}
		}
		if _, err := hijacked.Conn.Write(data); err != nil {
			break
		}
	}

	<-done
}
