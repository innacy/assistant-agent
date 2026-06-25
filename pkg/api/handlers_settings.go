package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/innacy/assistant-agent/internal/models"
)

func (s *Server) handleGetSettings(c *gin.Context) {
	settings, err := s.db.GetSettings(c.Request.Context(), defaultUserID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (s *Server) handleUpdateSettings(c *gin.Context) {
	var req models.Settings
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	req.UserID = defaultUserID

	if err := s.db.UpdateSettings(c.Request.Context(), &req); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c)
}
