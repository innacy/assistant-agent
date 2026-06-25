package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleListHistory(c *gin.Context) {
	filter := parseAlertFilter(c)
	result, err := s.db.ListHistory(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}
