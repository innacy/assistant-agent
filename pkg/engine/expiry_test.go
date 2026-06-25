package engine

import (
	"testing"
	"time"

	"github.com/innacy/assistant-agent/internal/models"
)

func TestComputeExpiresAt(t *testing.T) {
	due := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	ttl := map[string]int{"birthday": 2, "payment": 7}

	got := ComputeExpiresAt(due, "birthday", ttl)
	want := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("birthday: got %v, want %v", got, want)
	}

	got = ComputeExpiresAt(due, "payment", ttl)
	want = time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("payment: got %v, want %v", got, want)
	}

	got = ComputeExpiresAt(due, "unknown_type", ttl)
	want = time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("unknown: got %v, want %v", got, want)
	}
}

func TestNextOccurrenceYearly(t *testing.T) {
	due := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	next := NextOccurrence(due, models.RecurrenceYearly)
	if next == nil {
		t.Fatal("expected non-nil")
	}
	if next.Month() != 3 || next.Day() != 15 {
		t.Errorf("got %v, want March 15", next)
	}
	if next.Before(time.Now()) {
		t.Errorf("next occurrence should be in the future")
	}
}
