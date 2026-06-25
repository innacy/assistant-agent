package api

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/innacy/assistant-agent/pkg/config"
	"github.com/innacy/assistant-agent/pkg/db"
)

//go:embed all:web_dist
var webFS embed.FS

type Server struct {
	router    *gin.Engine
	db        *db.MongoDB
	cfg       *config.Config
	triggerCh chan struct{}
	isSyncing func() bool
}

func NewServer(database *db.MongoDB, cfg *config.Config, triggerCh chan struct{}, isSyncing func() bool) *Server {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(CORS())
	router.Use(RequestLogger())

	s := &Server{
		router:    router,
		db:        database,
		cfg:       cfg,
		triggerCh: triggerCh,
		isSyncing: isSyncing,
	}

	router.GET("/health", s.handleHealth)

	api := router.Group("/api")
	api.Use(BearerAuth(cfg.Server.APIToken))
	{
		api.GET("/alerts", s.handleListAlerts)
		api.GET("/alerts/upcoming", s.handleUpcomingAlerts)
		api.GET("/alerts/missed", s.handleMissedAlerts)
		api.GET("/alerts/today", s.handleTodayAlerts)
		api.GET("/alerts/:id", s.handleGetAlert)
		api.POST("/alerts", s.handleCreateAlert)
		api.PUT("/alerts/:id", s.handleUpdateAlert)
		api.DELETE("/alerts/:id", s.handleDeleteAlert)
		api.POST("/alerts/:id/acknowledge", s.handleAcknowledge)
		api.POST("/alerts/:id/snooze", s.handleSnooze)
		api.POST("/alerts/batch/acknowledge", s.handleBatchAcknowledge)
		api.POST("/alerts/batch/snooze", s.handleBatchSnooze)
		api.GET("/history", s.handleListHistory)
		api.GET("/sync/status", s.handleSyncStatus)
		api.POST("/sync/trigger", s.handleSyncTrigger)
		api.GET("/settings", s.handleGetSettings)
		api.PUT("/settings", s.handleUpdateSettings)
	}

	s.serveSPA(router)

	return s
}

func (s *Server) serveSPA(router *gin.Engine) {
	distFS, err := fs.Sub(webFS, "web_dist")
	if err != nil {
		return
	}

	fileServer := http.FileServer(http.FS(distFS))

	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		if strings.HasPrefix(path, "/api/") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}

		filePath := strings.TrimPrefix(path, "/")
		if filePath == "" {
			filePath = "index.html"
		}

		if f, err := distFS.Open(filePath); err == nil {
			f.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}

func (s *Server) Run() error {
	addr := fmt.Sprintf(":%d", s.cfg.Server.Port)
	return s.router.Run(addr)
}

func (s *Server) RunWithContext(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "ok",
		"service": "assistant-agent",
	})
}
