package gmail

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/sources"
)

var (
	amountRegex = regexp.MustCompile(`(?i)(?:₹|INR\s+|Rs\.?\s?)(\d[\d,]*\.?\d*)`)
	dateRegex   = regexp.MustCompile(`(\d{1,2})\s*(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\w*\s*(\d{4})?`)

	subjectPatterns = []string{
		"subscription", "renewal", "payment due", "bill generated",
		"upcoming charge", "reminder", "invoice", "due date",
		"your plan", "auto-renewal", "expiring",
	}
)

func ParseEmail(msgID, from, subject, body string, whitelistedSenders []string) *sources.RawItem {
	isWhitelisted := isSenderWhitelisted(from, whitelistedSenders)

	if !isWhitelisted {
		if !matchesSubjectPatterns(subject) {
			return nil
		}
		if !validateBody(body) {
			return nil
		}
	}

	title := extractServiceName(from, subject)
	amount := extractAmount(body + " " + subject)
	dueDate := extractDate(body + " " + subject)
	if dueDate.IsZero() {
		dueDate = time.Now().AddDate(0, 0, 3)
	}

	alertType := classifyEmail(subject, body)

	return &sources.RawItem{
		Type:        alertType,
		Title:       title,
		Description: truncate(subject, 200),
		DueDate:     dueDate,
		Amount:      amount,
		Currency:    "INR",
		Recurrence:  inferEmailRecurrence(subject, body),
		SourceRef:   msgID,
		SourceRaw:   subject,
		Priority:    models.PriorityMedium,
		Metadata: map[string]interface{}{
			"sender":      from,
			"whitelisted": isWhitelisted,
		},
	}
}

func isSenderWhitelisted(from string, whitelist []string) bool {
	lower := strings.ToLower(from)
	for _, s := range whitelist {
		if strings.Contains(lower, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

func matchesSubjectPatterns(subject string) bool {
	lower := strings.ToLower(subject)
	for _, pattern := range subjectPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func validateBody(body string) bool {
	if amountRegex.MatchString(body) {
		return true
	}
	if dateRegex.MatchString(body) {
		return true
	}
	lower := strings.ToLower(body)
	keywords := []string{"renewal", "expiry", "expires", "due date", "overdue", "payment"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func extractServiceName(from, subject string) string {
	parts := strings.SplitN(from, "<", 2)
	name := strings.TrimSpace(parts[0])
	if name != "" && name != from {
		return name
	}
	words := strings.Fields(subject)
	if len(words) > 5 {
		return strings.Join(words[:5], " ")
	}
	return subject
}

func extractAmount(text string) *float64 {
	match := amountRegex.FindStringSubmatch(text)
	if match == nil {
		return nil
	}
	cleaned := strings.ReplaceAll(match[1], ",", "")
	var val float64
	fmt.Sscanf(cleaned, "%f", &val)
	if val > 0 {
		return &val
	}
	return nil
}

func extractDate(text string) time.Time {
	match := dateRegex.FindStringSubmatch(text)
	if match == nil {
		return time.Time{}
	}
	dateStr := match[0]
	formats := []string{"2 Jan 2006", "02 Jan 2006", "2 January 2006", "02 January 2006"}
	for _, f := range formats {
		if t, err := time.Parse(f, dateStr); err == nil {
			if t.Year() == 0 {
				t = t.AddDate(time.Now().Year(), 0, 0)
			}
			return t
		}
	}
	return time.Time{}
}

func classifyEmail(subject, body string) string {
	combined := strings.ToLower(subject + " " + body)
	if strings.Contains(combined, "subscription") || strings.Contains(combined, "renewal") || strings.Contains(combined, "auto-renew") {
		return models.AlertTypeSubscription
	}
	return models.AlertTypePayment
}

func inferEmailRecurrence(subject, body string) string {
	combined := strings.ToLower(subject + " " + body)
	if strings.Contains(combined, "monthly") {
		return models.RecurrenceMonthly
	}
	if strings.Contains(combined, "yearly") || strings.Contains(combined, "annual") {
		return models.RecurrenceYearly
	}
	return models.RecurrenceNone
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
