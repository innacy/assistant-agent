package engine

import (
	"context"
	"math"
	"strings"
	"time"
	"unicode"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/db"
)

const (
	dateProximityDays   = 2
	jaccardThreshold    = 0.5
	amountTolerancePct  = 0.05
)

var titlePrefixes = []string{
	"re:", "fwd:", "fw:", "reminder:", "payment for",
}

func FindCrossSourceDuplicate(ctx context.Context, database *db.MongoDB, alert *models.Alert) (*models.Alert, error) {
	dayBefore := alert.DueDate.Add(-dateProximityDays * 24 * time.Hour)
	dayAfter := alert.DueDate.Add(dateProximityDays * 24 * time.Hour)

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
		if AlertsMatch(&existing, alert) {
			return &existing, nil
		}
	}

	return nil, nil
}

// AlertsMatch reports whether two alerts from different sources represent the same item.
func AlertsMatch(a, b *models.Alert) bool {
	if a.Type != b.Type {
		return false
	}
	if a.Source == b.Source {
		return false
	}
	if !datesWithinProximity(a.DueDate, b.DueDate, dateProximityDays) {
		return false
	}
	if !amountsMatch(a.Amount, b.Amount) {
		return false
	}
	return titleSimilar(normalizeTitle(a.Title), normalizeTitle(b.Title))
}

func normalizeTitle(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	for {
		changed := false
		for _, prefix := range titlePrefixes {
			if strings.HasPrefix(s, prefix) {
				s = strings.TrimSpace(s[len(prefix):])
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return s
}

func tokenize(title string) map[string]struct{} {
	tokens := make(map[string]struct{})
	for _, word := range strings.FieldsFunc(title, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		word = strings.TrimSpace(word)
		if word != "" {
			tokens[word] = struct{}{}
		}
	}
	return tokens
}

func jaccardSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	tokensA := tokenize(a)
	tokensB := tokenize(b)
	if len(tokensA) == 0 && len(tokensB) == 0 {
		return 1.0
	}
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0.0
	}

	intersection := 0
	for t := range tokensA {
		if _, ok := tokensB[t]; ok {
			intersection++
		}
	}
	union := len(tokensA) + len(tokensB) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

func titleSimilar(a, b string) bool {
	if a == b {
		return true
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}
	return jaccardSimilarity(a, b) >= jaccardThreshold
}

func datesWithinProximity(a, b time.Time, days int) bool {
	diff := a.Sub(b)
	if diff < 0 {
		diff = -diff
	}
	return diff <= time.Duration(days)*24*time.Hour
}

func amountsMatch(a, b *float64) bool {
	if a == nil || b == nil {
		return true
	}
	if *a == *b {
		return true
	}
	maxVal := math.Max(math.Abs(*a), math.Abs(*b))
	if maxVal == 0 {
		return *a == *b
	}
	diff := math.Abs(*a - *b)
	return diff/maxVal <= amountTolerancePct
}

func sourcePriority(source string) int {
	switch source {
	case models.SourceGmail:
		return 4
	case models.SourceCalendar:
		return 3
	case models.SourceTasks:
		return 2
	case models.SourceContacts:
		return 1
	default:
		return 0
	}
}

// MergeCrossSourceAlerts keeps the richer source as primary and records the other in metadata.merged_from.
func MergeCrossSourceAlerts(existing, incoming *models.Alert) *models.Alert {
	var primary, secondary *models.Alert
	if sourcePriority(incoming.Source) > sourcePriority(existing.Source) {
		primary = incoming
		secondary = existing
	} else {
		primary = existing
		secondary = incoming
	}

	merged := *primary
	merged.ID = existing.ID
	if merged.Metadata == nil {
		merged.Metadata = make(map[string]interface{})
	}

	mergedFrom, _ := merged.Metadata["merged_from"].([]interface{})
	entry := map[string]interface{}{
		"source":     secondary.Source,
		"source_ref": secondary.SourceRef,
		"title":      secondary.Title,
	}
	merged.Metadata["merged_from"] = append(mergedFrom, entry)

	if merged.Amount == nil && secondary.Amount != nil {
		merged.Amount = secondary.Amount
		if merged.Currency == "" {
			merged.Currency = secondary.Currency
		}
	}
	if merged.Description == "" && secondary.Description != "" {
		merged.Description = secondary.Description
	}

	return &merged
}
