package api

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/innacy/assistant-agent/pkg/config"
	"github.com/innacy/assistant-agent/pkg/db"
)

type Server struct {
	router *gin.Engine
	db     *db.MongoDB
	cfg    *config.Config
}

func NewServer(database *db.MongoDB, cfg *config.Config) *Server {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(CORS())
	router.Use(RequestLogger())

	s := &Server{
		router: router,
		db:     database,
		cfg:    cfg,
	}

	router.GET("/health", s.handleHealth)

	api := router.Group("/api")
	api.Use(BearerAuth(cfg.Server.APIToken))
	{
		// Alert endpoints will be registered here
	}

	return s
}

func (s *Server) Run() error {
	addr := fmt.Sprintf(":%d", s.cfg.Server.Port)
	return s.router.Run(addr)
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "ok",
		"service": "assistant-agent",
	})
}
