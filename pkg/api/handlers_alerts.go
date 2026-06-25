package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/db"
	"github.com/innacy/assistant-agent/pkg/engine"
)

const defaultUserID = "default"

func (s *Server) handleListAlerts(c *gin.Context) {
	filter := parseAlertFilter(c)
	result, err := s.db.ListAlerts(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	result.Data = engine.RecomputeStatuses(result.Data)
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleGetAlert(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}

	alert, err := s.db.GetAlert(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusNotFound, "alert not found")
		return
	}
	alert.Status = engine.ComputeStatus(alert, time.Now())
	c.JSON(http.StatusOK, alert)
}

func (s *Server) handleUpcomingAlerts(c *gin.Context) {
	filter := db.AlertFilter{
		UserID: defaultUserID,
		Status: []string{models.AlertStatusUpcoming},
		Limit:  50,
	}
	result, err := s.db.ListAlerts(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	result.Data = engine.RecomputeStatuses(result.Data)
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleMissedAlerts(c *gin.Context) {
	filter := db.AlertFilter{
		UserID: defaultUserID,
		Status: []string{models.AlertStatusMissed},
		Limit:  50,
	}
	result, err := s.db.ListAlerts(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	result.Data = engine.RecomputeStatuses(result.Data)
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleTodayAlerts(c *gin.Context) {
	filter := db.AlertFilter{
		UserID: defaultUserID,
		Status: []string{models.AlertStatusDueToday},
		Limit:  50,
	}
	result, err := s.db.ListAlerts(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	result.Data = engine.RecomputeStatuses(result.Data)
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleCreateAlert(c *gin.Context) {
	var req struct {
		Type        string   `json:"type" binding:"required"`
		Title       string   `json:"title" binding:"required"`
		Description string   `json:"description"`
		DueDate     string   `json:"due_date" binding:"required"`
		Recurrence  string   `json:"recurrence"`
		Amount      *float64 `json:"amount"`
		Priority    string   `json:"priority"`
		Tags        []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	dueDate, err := time.Parse("2006-01-02", req.DueDate)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid due_date format (use YYYY-MM-DD)")
		return
	}

	settings, _ := s.db.GetSettings(c.Request.Context(), defaultUserID)
	windows := s.cfg.Alerts.Windows
	ttl := s.cfg.Alerts.TTL
	if settings != nil {
		windows = settings.Windows
		ttl = settings.TTL
	}

	recurrence := req.Recurrence
	if recurrence == "" {
		recurrence = models.RecurrenceNone
	}
	priority := req.Priority
	if priority == "" {
		priority = models.PriorityMedium
	}

	now := time.Now()
	alert := models.Alert{
		UserID:       defaultUserID,
		Type:         req.Type,
		Title:        req.Title,
		Description:  req.Description,
		DueDate:      dueDate,
		Recurrence:   recurrence,
		Amount:       req.Amount,
		Currency:     "INR",
		Source:       models.SourceManual,
		SourceRef:    "manual:" + uuid.New().String(),
		Priority:     priority,
		WindowBefore: engine.ComputeWindowBefore(req.Type, windows),
		ExpiresAt:    engine.ComputeExpiresAt(dueDate, req.Type, ttl),
		Tags:         req.Tags,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	alert.Status = engine.ComputeStatus(&alert, now)
	if recurrence != models.RecurrenceNone {
		alert.NextOccurrence = engine.NextOccurrence(dueDate, recurrence)
	}

	if err := s.db.UpsertAlert(c.Request.Context(), &alert); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, alert)
}

func (s *Server) handleUpdateAlert(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}

	alert, err := s.db.GetAlert(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusNotFound, "alert not found")
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if v, ok := req["title"].(string); ok {
		alert.Title = v
	}
	if v, ok := req["description"].(string); ok {
		alert.Description = v
	}
	if v, ok := req["due_date"].(string); ok {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			alert.DueDate = t
		}
	}
	if v, ok := req["priority"].(string); ok {
		alert.Priority = v
	}
	if v, ok := req["tags"].([]interface{}); ok {
		tags := make([]string, len(v))
		for i, t := range v {
			tags[i], _ = t.(string)
		}
		alert.Tags = tags
	}
	alert.UpdatedAt = time.Now()
	alert.Status = engine.ComputeStatus(alert, time.Now())

	if err := s.db.UpsertAlert(c.Request.Context(), alert); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, alert)
}

func (s *Server) handleDeleteAlert(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.DeleteAlert(c.Request.Context(), id); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c)
}

func (s *Server) handleAcknowledge(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}

	alert, err := s.db.GetAlert(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusNotFound, "alert not found")
		return
	}

	now := time.Now()
	alert.Status = models.AlertStatusAcknowledged
	alert.AcknowledgedAt = &now
	if err := s.db.ArchiveAlert(c.Request.Context(), alert, "acknowledged"); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c)
}

func (s *Server) handleSnooze(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}

	var req struct {
		SnoozeUntil string `json:"snooze_until" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	until, err := time.Parse("2006-01-02", req.SnoozeUntil)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid snooze_until date (use YYYY-MM-DD)")
		return
	}

	err = s.db.UpdateAlertStatus(c.Request.Context(), id, models.AlertStatusSnoozed, bson.M{
		"snoozed_until": until,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c)
}

func (s *Server) handleBatchAcknowledge(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx := c.Request.Context()
	now := time.Now()
	var count int64

	ids := make([]primitive.ObjectID, 0, len(req.IDs))
	for _, idStr := range req.IDs {
		if id, err := primitive.ObjectIDFromHex(idStr); err == nil {
			ids = append(ids, id)
		}
	}

	for _, id := range ids {
		alert, err := s.db.GetAlert(ctx, id)
		if err != nil {
			continue
		}
		alert.Status = models.AlertStatusAcknowledged
		alert.AcknowledgedAt = &now
		if err := s.db.ArchiveAlert(ctx, alert, "acknowledged"); err == nil {
			count++
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "count": count})
}

func (s *Server) handleBatchSnooze(c *gin.Context) {
	var req struct {
		IDs         []string `json:"ids" binding:"required"`
		SnoozeUntil string   `json:"snooze_until" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	until, err := time.Parse("2006-01-02", req.SnoozeUntil)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid snooze_until date (use YYYY-MM-DD)")
		return
	}

	ctx := c.Request.Context()
	ids := make([]primitive.ObjectID, 0, len(req.IDs))
	for _, idStr := range req.IDs {
		if id, err := primitive.ObjectIDFromHex(idStr); err == nil {
			ids = append(ids, id)
		}
	}

	n, err := s.db.BulkUpdateStatus(ctx, bson.M{"_id": bson.M{"$in": ids}}, models.AlertStatusSnoozed, bson.M{
		"snoozed_until": until,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "count": n})
}

func parseAlertFilter(c *gin.Context) db.AlertFilter {
	f := db.AlertFilter{UserID: defaultUserID}

	if types := c.Query("type"); types != "" {
		f.Types = strings.Split(types, ",")
	}
	if status := c.Query("status"); status != "" {
		f.Status = strings.Split(status, ",")
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			f.From = &t
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			f.To = &t
		}
	}
	if limit := c.Query("limit"); limit != "" {
		if n, err := strconv.ParseInt(limit, 10, 64); err == nil {
			f.Limit = n
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if n, err := strconv.ParseInt(offset, 10, 64); err == nil {
			f.Offset = n
		}
	}

	return f
}
