package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ListResponse struct {
	Data    interface{} `json:"data"`
	Total   int64       `json:"total"`
	Limit   int64       `json:"limit"`
	Offset  int64       `json:"offset"`
	HasMore bool        `json:"has_more"`
}

func respondOK(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}
