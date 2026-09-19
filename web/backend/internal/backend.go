package internal

import (
	"backend/config"
	flctx "backend/internal/context"
	"backend/internal/processor"
	"backend/logger"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	loggergo "github.com/Alonza0314/logger-go/v2"
	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
)

type jwt struct {
	secret    string
	expiresIn time.Duration
}

type backend struct {
	router *gin.Engine
	server *http.Server

	username string
	password string

	port int

	jwt

	frontendFilePath string

	processor.Processor

	*logger.BackendLogger
}

func NewBackend(config *config.Config, logger *logger.BackendLogger) *backend {
	flCtx := flctx.NewFlContext(&flctx.FlCotextIE{
		DbType: config.Backend.Db.Type,
		DbPath: config.Backend.Db.Path,

		DeployWorkDir: config.Backend.Deploy.WorkDir,

		BackendLogger: logger,
	})
	if flCtx == nil {
		logger.BckLog.Errorln("Failed to create FlContext")
		return nil
	}

	b := &backend{
		router: nil,
		server: nil,

		username: config.Backend.Username,
		password: config.Backend.Password,

		port: config.Backend.Port,

		jwt: jwt{
			secret:    config.Backend.JWT.Secret,
			expiresIn: config.Backend.JWT.ExpiresIn,
		},

		frontendFilePath: config.Backend.FrontendFilePath,

		Processor: *processor.NewProcessor(&processor.ProcessorIE{
			Username: config.Backend.Username,
			Password: config.Backend.Password,

			JwtSecret:    config.Backend.JWT.Secret,
			JwtExpiresIn: config.Backend.JWT.ExpiresIn,

			FlContext: flCtx,

			BackendLogger: logger,
		}),

		BackendLogger: logger,
	}

	gin.DefaultWriter, gin.DefaultErrorWriter = loggergo.NewGinWriter(logger.GinLog), loggergo.NewGinWriter(logger.GinLog)

	b.router = util.NewGinRouter("", nil)

	b.router.UseRawPath = true
	b.router.NoRoute(b.returnPages())

	addServices(b.router, b)
	addMiddleware(b.router)

	return b
}

func (b *backend) returnPages() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method == http.MethodGet {

			destPath := filepath.Join(b.frontendFilePath, c.Request.URL.Path)
			if stat, err := os.Stat(destPath); err == nil && !stat.IsDir() {
				c.File(filepath.Clean(destPath))
				return
			}

			c.File(filepath.Clean(filepath.Join(b.frontendFilePath, "index.html")))
		} else {
			c.Next()
		}
	}
}

func (b *backend) Start() {
	b.BckLog.Infoln("Starting backend server...")

	b.server = &http.Server{
		Addr:    ":" + strconv.Itoa(b.port),
		Handler: b.router,
	}

	go func() {
		if err := b.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			b.BckLog.Errorf("Failed to start server: %s\n", err)
		}
	}()
	time.Sleep(500 * time.Millisecond)

	b.BckLog.Infof("Backend server started on port: %d", b.port)
}

func (b *backend) Stop() {
	fmt.Println()
	b.BckLog.Infoln("Stopping backend server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := b.server.Shutdown(shutdownCtx); err != nil {
		b.BckLog.Errorf("Failed to stop backend server: %v", err)
	} else {
		b.BckLog.Infoln("Backend server stopped successfully")
	}

	b.Processor.Release()
}

func addServices(router *gin.Engine, b *backend) {
	router.RedirectTrailingSlash = false

	apiGroup := router.Group("/api")

	authGroup := apiGroup.Group("")
	authGroup.Use(addAuthMiddleware(b))

	addRoutes(apiGroup, b.getAccountRoutes())
	addRoutes(authGroup, b.getDeployRoutes())
}

func addRoutes(group *gin.RouterGroup, routes util.Routes) {
	for _, route := range routes {
		switch route.Method {
		case "GET":
			group.GET(route.Pattern, route.HandlerFunc)
		case "POST":
			group.POST(route.Pattern, route.HandlerFunc)
		case "PUT":
			group.PUT(route.Pattern, route.HandlerFunc)
		case "DELETE":
			group.DELETE(route.Pattern, route.HandlerFunc)
		case "PATCH":
			group.PATCH(route.Pattern, route.HandlerFunc)
		}
	}
}
