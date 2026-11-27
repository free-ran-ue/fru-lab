package internal

import (
	"backend/constant"
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"sync"

	flctx "backend/internal/context"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Live streams for the workbench console, as websockets (a browser cannot
// set headers on one, so the JWT comes as ?token=, like the terminal):
//
//	/stream/logs     follows a target's logs, one JSON LogLine per message
//	/stream/ue-test  runs a ping / curl / iperf3 from a UE over its tunnel
func (b *backend) getStreamRoutes() util.Routes {
	return util.Routes{
		{Name: "StreamLogs", Method: http.MethodGet, Pattern: "/stream/logs", HandlerFunc: b.handleStreamLogs},
		{Name: "StreamUeTest", Method: http.MethodGet, Pattern: "/stream/ue-test", HandlerFunc: b.handleStreamUeTest},
	}
}

var streamTargets = map[string]bool{
	constant.DEPLOY_TARGET_FREE5GC:    true,
	constant.DEPLOY_TARGET_GNB:        true,
	constant.DEPLOY_TARGET_GNB_SLICE1: true,
	constant.DEPLOY_TARGET_GNB_SLICE2: true,
	constant.DEPLOY_TARGET_UE:         true,
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

func (b *backend) handleStreamLogs(c *gin.Context) {
	target, instance := c.Query("target"), c.Query("instance")
	if !streamTargets[target] || (target == constant.DEPLOY_TARGET_UE && instance == "") {
		c.JSON(http.StatusBadRequest, gin.H{"message": "unknown target"})
		return
	}
	tail, err := strconv.Atoi(c.DefaultQuery("tail", "200"))
	if err != nil || tail < 0 || tail > 5000 {
		tail = 200
	}
	conn := b.upgradeAuthed(c)
	if conn == nil {
		return
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	closeOnHangup(conn, cancel)

	var mu sync.Mutex
	err = b.FlContext.FollowLogs(ctx, target, instance, parseServiceQuery(c), tail, func(l flctx.LogLine) {
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

var (
	reHost   = regexp.MustCompile(`^[A-Za-z0-9.:-]{1,253}$`)
	reURL    = regexp.MustCompile(`^https?://[A-Za-z0-9.:/?&=%_~+-]{1,400}$`)
	reTunDev = regexp.MustCompile(`^ueTun[0-9]{0,3}$`)
)

// free-ran-ue names the tunnel after its config's ueTunnelDevice plus an
// index ("ueTun0"); the frontend reads the real name from the UE's log.
func tunDev(q func(string) string) string {
	if dev := q("dev"); reTunDev.MatchString(dev) {
		return dev
	}
	return "ueTun0"
}

// ueTestCommand builds one of the allowed test commands, sent over the
// UE's tunnel; anything else is refused.
func ueTestCommand(q func(string) string) ([]string, string) {
	ueTunDev := tunDev(q)
	switch q("action") {
	case "ping":
		host := q("target")
		count, _ := strconv.Atoi(q("count"))
		if count < 1 || count > 50 {
			count = 4
		}
		if !reHost.MatchString(host) {
			return nil, "target must be a host name or IP"
		}
		return []string{"ping", "-I", ueTunDev, "-c", strconv.Itoa(count), "-W", "2", host}, ""
	case "curl":
		url := q("target")
		if !reURL.MatchString(url) {
			return nil, "target must be an http(s) URL"
		}
		return []string{"curl", "--interface", ueTunDev, "-sS", "-o", "/dev/null", "-m", "15",
			"-w", "HTTP %{http_code}  dns %{time_namelookup}s  connect %{time_connect}s  total %{time_total}s  %{size_download} bytes  %{speed_download} B/s\n", url}, ""
	case "iperf3":
		host := q("target")
		secs, _ := strconv.Atoi(q("seconds"))
		if secs < 1 || secs > 60 {
			secs = 5
		}
		if !reHost.MatchString(host) {
			return nil, "target must be an iperf3 server host or IP"
		}
		cmd := []string{"iperf3", "-c", host, "--bind-dev", ueTunDev, "-t", strconv.Itoa(secs), "-f", "m"}
		if q("reverse") == "1" {
			cmd = append(cmd, "-R")
		}
		return cmd, ""
	}
	return nil, "action must be ping, curl or iperf3"
}

func (b *backend) handleStreamUeTest(c *gin.Context) {
	instance := c.Query("instance")
	cmd, problem := ueTestCommand(c.Query)
	if instance == "" || problem != "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": problem})
		return
	}
	conn := b.upgradeAuthed(c)
	if conn == nil {
		return
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	closeOnHangup(conn, cancel)

	_ = conn.WriteMessage(websocket.TextMessage, []byte("$ "+joinCmd(cmd)+"\r\n"))
	hijacked, err := b.FlContext.ExecUe(ctx, instance, cmd)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(err.Error()+"\r\n"))
		return
	}
	defer hijacked.Close()
	go func() {
		<-ctx.Done()
		hijacked.Close()
	}()

	b.DeployLog.Infof("UE test on %s: %s", instance, joinCmd(cmd))
	buf := make([]byte, 4096)
	for {
		n, err := hijacked.Reader.Read(buf)
		if n > 0 && conn.WriteMessage(websocket.BinaryMessage, buf[:n]) != nil {
			return
		}
		if err != nil {
			return
		}
	}
}

func joinCmd(cmd []string) string {
	out := ""
	for i, a := range cmd {
		if i > 0 {
			out += " "
		}
		if regexp.MustCompile(`[\s%]`).MatchString(a) {
			a = "'" + a + "'"
		}
		out += a
	}
	return out
}
