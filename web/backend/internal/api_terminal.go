package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	dockerclient "github.com/moby/moby/client"
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
		{
			Name:        "DeployGnbTerminal",
			Method:      http.MethodGet,
			Pattern:     "/deploy/gnbs/:profileId/terminal",
			HandlerFunc: b.handleGnbTerminal,
		},
	}
}

func (b *backend) handleUeTerminal(c *gin.Context) {
	instance := c.Param("instance")
	b.serveTerminal(c, instance, func(ctx context.Context) (dockerclient.HijackedResponse, string, error) {
		return b.FlContext.AttachUeShell(ctx, instance)
	})
}

var reProfileID = regexp.MustCompile(`^[0-9a-f]{12}$`)

// handleGnbTerminal is a shell in the running gNB of one gNB profile.
func (b *backend) handleGnbTerminal(c *gin.Context) {
	profileID := c.Param("profileId")
	if !reProfileID.MatchString(profileID) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid gNB profile id"})
		return
	}
	b.serveTerminal(c, "gnb-"+profileID, func(ctx context.Context) (dockerclient.HijackedResponse, string, error) {
		return b.FlContext.AttachGnbShell(ctx, profileID)
	})
}

// serveTerminal bridges a websocket and a shell attached by attach.
func (b *backend) serveTerminal(c *gin.Context, instance string, attach func(context.Context) (dockerclient.HijackedResponse, string, error)) {
	if _, err := util.ValidateJWT(c.Query("token"), b.jwt.secret); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid token: " + err.Error()})
		return
	}

	conn, err := terminalUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		b.DeployLog.Warnf("Failed to upgrade websocket for terminal %s: %v", instance, err)
		return
	}
	// closed here, or already when the shell ended
	defer func() { _ = conn.Close() }()

	hijacked, execID, err := attach(c.Request.Context())
	if err != nil {
		b.DeployLog.Warnf("Failed to attach terminal for %s: %v", instance, err)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n["+err.Error()+"]\r\n"))
		return
	}
	defer hijacked.Close()

	b.DeployLog.Infof("Terminal attached for %s (exec %s)", instance, execID)

	done := make(chan struct{})

	// container -> browser
	go func() {
		defer close(done)
		// the shell ended (exit, or the container stopped): close the
		// websocket too, which also ends the read loop below and tells the
		// browser to close its terminal. WriteControl and Close are safe
		// next to the other goroutine's reads and writes.
		defer func() {
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shell exited"), time.Now().Add(time.Second))
			_ = conn.Close()
		}()
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
					b.DeployLog.Warnf("Failed to resize terminal for %s: %v", instance, err)
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
