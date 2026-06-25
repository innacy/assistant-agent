package engine

import (
	"testing"
	"time"

	"github.com/innacy/assistant-agent/internal/models"
)

func TestAlertsMatch_NetflixMonthlyAndNetflix(t *testing.T) {
	due := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	gmail := &models.Alert{
		Type:    models.AlertTypeSubscription,
		Title:   "Netflix Monthly",
		Source:  models.SourceGmail,
		DueDate: due,
	}
	calendar := &models.Alert{
		Type:    models.AlertTypeSubscription,
		Title:   "Netflix",
		Source:  models.SourceCalendar,
		DueDate: due,
	}
	if !AlertsMatch(gmail, calendar) {
		t.Fatal("expected Netflix Monthly (gmail) to match Netflix (calendar)")
	}
}

func TestAlertsMatch_DifferentAmountsNotDeduped(t *testing.T) {
	due := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	amount499 := 499.0
	amount999 := 999.0
	a := &models.Alert{
		Type:    models.AlertTypeSubscription,
		Title:   "Spotify Premium",
		Source:  models.SourceGmail,
		DueDate: due,
		Amount:  &amount499,
	}
	b := &models.Alert{
		Type:    models.AlertTypeSubscription,
		Title:   "Spotify Premium",
		Source:  models.SourceCalendar,
		DueDate: due,
		Amount:  &amount999,
	}
	if AlertsMatch(a, b) {
		t.Fatal("expected different amounts to prevent dedup")
	}
}

func TestNormalizeTitle_StripsREPrefix(t *testing.T) {
	a := normalizeTitle("RE: Payment reminder")
	b := normalizeTitle("payment reminder")
	if a != b {
		t.Fatalf("got %q, want %q", a, b)
	}
	if !titleSimilar(a, b) {
		t.Fatal("expected normalized titles to match")
	}
}

func TestAlertsMatch_CompletelyDifferentTitles(t *testing.T) {
	due := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	a := &models.Alert{
		Type:    models.AlertTypePayment,
		Title:   "Electricity Bill",
		Source:  models.SourceGmail,
		DueDate: due,
	}
	b := &models.Alert{
		Type:    models.AlertTypePayment,
		Title:   "Grocery Shopping",
		Source:  models.SourceCalendar,
		DueDate: due,
	}
	if AlertsMatch(a, b) {
		t.Fatal("expected unrelated titles not to match")
	}
}

func TestAlertsMatch_DatesThreeDaysApartNotDeduped(t *testing.T) {
	a := &models.Alert{
		Type:    models.AlertTypeSubscription,
		Title:   "Netflix",
		Source:  models.SourceGmail,
		DueDate: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
	}
	b := &models.Alert{
		Type:    models.AlertTypeSubscription,
		Title:   "Netflix",
		Source:  models.SourceCalendar,
		DueDate: time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC),
	}
	if AlertsMatch(a, b) {
		t.Fatal("expected dates 3 days apart not to match")
	}
}

func TestMergeCrossSourceAlerts_PrefersGmail(t *testing.T) {
	existing := &models.Alert{
		Source:    models.SourceCalendar,
		SourceRef: "cal:1",
		Title:     "Netflix",
	}
	incoming := &models.Alert{
		Source:    models.SourceGmail,
		SourceRef: "gmail:1",
		Title:     "Netflix Monthly",
	}
	merged := MergeCrossSourceAlerts(existing, incoming)
	if merged.Source != models.SourceGmail {
		t.Fatalf("expected gmail as primary, got %s", merged.Source)
	}
	mergedFrom, ok := merged.Metadata["merged_from"].([]interface{})
	if !ok || len(mergedFrom) != 1 {
		t.Fatalf("expected merged_from entry, got %v", merged.Metadata["merged_from"])
	}
}
