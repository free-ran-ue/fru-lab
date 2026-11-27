package internal

import (
	"backend/constant"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"

	flctx "backend/internal/context"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Live log streams, as websockets (a browser cannot set headers on one, so
// the JWT comes as ?token=, like the terminal): /stream/logs follows a
// target's logs, one JSON LogLine per message.
func (b *backend) getStreamRoutes() util.Routes {
	return util.Routes{
		{Name: "StreamLogs", Method: http.MethodGet, Pattern: "/stream/logs", HandlerFunc: b.handleStreamLogs},
	}
}

var streamTargets = map[string]bool{
	constant.DEPLOY_TARGET_FREE5GC: true,
	constant.DEPLOY_TARGET_UE:      true,
}

// validStreamTarget tells whether logs of target (and, for "ue", instance)
// can be streamed: free5gc, a profile gNB, gnb-<profile id>, or a free5gc
// core network, core-<core id>.
func validStreamTarget(target, instance string) bool {
	if target == constant.DEPLOY_TARGET_UE {
		return instance != ""
	}
	return streamTargets[target] || flctx.IsGnbInstance(target) || flctx.IsCoreInstance(target)
}

func (b *backend) upgradeAuthed(c *gin.Context) *websocket.Conn {
	if _, err := util.ValidateJWT(c.Query("token"), b.jwt.secret); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid token: " + err.Error()})
		return nil
	}
	conn, err := terminalUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		b.DeployLog.Warnf("Failed to upgrade websocket: %v", err)
		return nil
	}
	return conn
}

// closeOnHangup cancels ctx when the browser closes the socket.
func closeOnHangup(conn *websocket.Conn, cancel context.CancelFunc) {
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

// parseTail reads the stream's tail query: a line count (0-5000, else
// 200), or "all" (-1) for everything since the container started - the
// event timeline replays it so it holds events from before the page opened.
func parseTail(q string) int {
	if q == "all" {
		return -1
	}
	n, err := strconv.Atoi(q)
	if err != nil || n < 0 || n > 5000 {
		return 200
	}
	return n
}

func (b *backend) handleStreamLogs(c *gin.Context) {
	target, instance := c.Query("target"), c.Query("instance")
	if !validStreamTarget(target, instance) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "unknown target"})
		return
	}
	tail := parseTail(c.DefaultQuery("tail", "200"))
	conn := b.upgradeAuthed(c)
	if conn == nil {
		return
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	closeOnHangup(conn, cancel)

	var mu sync.Mutex
	err := b.FlContext.FollowLogs(ctx, target, instance, parseServiceQuery(c), tail, func(l flctx.LogLine) {
		raw, _ := json.Marshal(l)
		mu.Lock()
		defer mu.Unlock()
		if conn.WriteMessage(websocket.TextMessage, raw) != nil {
			cancel()
		}
	})
	if err != nil && ctx.Err() == nil {
		raw, _ := json.Marshal(gin.H{"error": err.Error()})
		_ = conn.WriteMessage(websocket.TextMessage, raw)
	}
}
