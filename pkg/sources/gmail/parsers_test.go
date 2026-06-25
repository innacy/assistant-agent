package gmail

import (
	"testing"
)

func TestIsSenderWhitelisted(t *testing.T) {
	whitelist := []string{"noreply@netflix.com", "alerts@hdfcbank.net"}

	if !isSenderWhitelisted("Netflix <noreply@netflix.com>", whitelist) {
		t.Error("should match netflix")
	}
	if isSenderWhitelisted("random@spam.com", whitelist) {
		t.Error("should not match spam")
	}
}

func TestMatchesSubjectPatterns(t *testing.T) {
	if !matchesSubjectPatterns("Your subscription has been renewed") {
		t.Error("should match subscription")
	}
	if !matchesSubjectPatterns("Payment due for your account") {
		t.Error("should match payment due")
	}
	if matchesSubjectPatterns("Check out our new products!") {
		t.Error("should not match marketing")
	}
}

func TestExtractAmount(t *testing.T) {
	tests := []struct {
		input string
		want  *float64
	}{
		{"Amount: ₹499", floatPtr(499)},
		{"INR 1,299.00 charged", floatPtr(1299)},
		{"Rs. 2500 deducted", floatPtr(2500)},
		{"No amount here", nil},
	}

	for _, tt := range tests {
		got := extractAmount(tt.input)
		if tt.want == nil && got != nil {
			t.Errorf("input=%q: want nil, got %v", tt.input, *got)
		}
		if tt.want != nil && (got == nil || *got != *tt.want) {
			t.Errorf("input=%q: want %v, got %v", tt.input, *tt.want, got)
		}
	}
}

func TestParseEmailWhitelistedSender(t *testing.T) {
	whitelist := []string{"noreply@netflix.com"}
	item := ParseEmail(
		"msg123",
		"Netflix <noreply@netflix.com>",
		"Your Netflix membership renewal",
		"Your subscription will renew on 15 Jul 2026 for ₹649",
		whitelist,
	)
	if item == nil {
		t.Fatal("expected non-nil item for whitelisted sender")
	}
	if item.Type != "subscription" {
		t.Errorf("type: got %q, want subscription", item.Type)
	}
	if item.Amount == nil || *item.Amount != 649 {
		t.Errorf("amount: got %v, want 649", item.Amount)
	}
}

func TestParseEmailUnknownSenderWithValidation(t *testing.T) {
	item := ParseEmail(
		"msg456",
		"billing@random-saas.com",
		"Your subscription renewal is coming up",
		"Your plan will auto-renew on 20 Aug 2026 for ₹999 per month",
		[]string{"noreply@netflix.com"},
	)
	if item == nil {
		t.Fatal("expected non-nil item (subject+body both valid)")
	}
}

func TestParseEmailRejectsSpam(t *testing.T) {
	item := ParseEmail(
		"msg789",
		"marketing@shop.com",
		"Big sale this weekend!",
		"Shop now and save 50% on all items",
		[]string{"noreply@netflix.com"},
	)
	if item != nil {
		t.Error("expected nil for spam email")
	}
}

func floatPtr(f float64) *float64 { return &f }
