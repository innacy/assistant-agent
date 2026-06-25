package engine

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/db"
)

func FindCrossSourceDuplicate(ctx context.Context, database *db.MongoDB, alert *models.Alert) (*models.Alert, error) {
	normalizedTitle := strings.ToLower(strings.TrimSpace(alert.Title))
	dayBefore := alert.DueDate.Add(-24 * time.Hour)
	dayAfter := alert.DueDate.Add(24 * time.Hour)

	filter := bson.M{
		"user_id": alert.UserID,
		"type":    alert.Type,
		"source":  bson.M{"$ne": alert.Source},
		"due_date": bson.M{
			"$gte": dayBefore,
			"$lte": dayAfter,
		},
	}

	cursor, err := database.Alerts().Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var existing models.Alert
		if err := cursor.Decode(&existing); err != nil {
			continue
		}
		existingTitle := strings.ToLower(strings.TrimSpace(existing.Title))
		if titleSimilar(normalizedTitle, existingTitle) {
			return &existing, nil
		}
	}

	return nil, nil
}

func titleSimilar(a, b string) bool {
	if a == b {
		return true
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}
	return false
}
