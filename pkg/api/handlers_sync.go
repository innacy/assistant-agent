package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/innacy/assistant-agent/internal/models"
)

func (s *Server) handleSyncStatus(c *gin.Context) {
	states, err := s.db.GetAllSyncStates(c.Request.Context(), defaultUserID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if states == nil {
		states = []models.SyncState{}
	}
	c.JSON(http.StatusOK, gin.H{"data": states})
}

func (s *Server) handleSyncTrigger(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "sync triggered"})
}
