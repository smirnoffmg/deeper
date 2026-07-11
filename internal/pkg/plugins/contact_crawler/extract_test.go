package contact_crawler

import (
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractEmails(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "single", text: "Contact us at info@example.com today", want: []string{"info@example.com"}},
		{name: "multiple", text: "a@b.co and c@d.org", want: []string{"a@b.co", "c@d.org"}},
		{name: "no match", text: "not an email @media screen", want: nil},
		{name: "version string", text: "version 1.2.3.4 release", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractEmails(tt.text))
		})
	}
}

func TestExtractPhones(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "us format", text: "Call (555) 123-4567", want: []string{"(555) 123-4567"}},
		{name: "intl", text: "Phone +1 555-123-4567", want: []string{"+1 555-123-4567"}},
		{name: "date false positive risk", text: "Updated 2024-01-15", want: nil},
		{name: "no match", text: "no numbers here", want: nil},
		// Regression: found live against codescoring.ru — bare unformatted
		// digit runs (Telegram user IDs, tax IDs) were being misclassified
		// as phone numbers purely because they happened to be 10-13 digits.
		{name: "bare telegram-id-shaped digits rejected", text: "user id 1737200008307 seen", want: nil},
		{name: "bare tax-id-shaped digits rejected", text: "INN 7813227385 registered", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPhones(tt.text)
			if tt.want == nil {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLooksLikePhone(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "plus prefix, no separators", value: "+15551234567", want: true},
		{name: "dashes", value: "555-123-4567", want: true},
		{name: "parens and space", value: "(555) 123-4567", want: true},
		{name: "dots", value: "555.123.4567", want: true},
		{name: "bare digits, no plus, no separators", value: "5551234567", want: false},
		{name: "bare 13-digit id", value: "1737200008307", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, looksLikePhone(tt.value))
		})
	}
}

func TestMailtoTrace(t *testing.T) {
	trace, ok := mailtoTrace("mailto:info@example.com?subject=Hi")
	require.True(t, ok)
	assert.Equal(t, entities.Email, trace.Type)
	assert.Equal(t, "info@example.com", trace.Value)

	_, ok = mailtoTrace("https://example.com")
	assert.False(t, ok)
}

func TestTelTrace(t *testing.T) {
	trace, ok := telTrace("tel:+1-555-123-4567")
	require.True(t, ok)
	assert.Equal(t, entities.Phone, trace.Type)
	assert.Equal(t, "+1-555-123-4567", trace.Value)

	_, ok = telTrace("mailto:x@y.com")
	assert.False(t, ok)
}

func TestSocialLinkTrace(t *testing.T) {
	tests := []struct {
		name      string
		href      string
		wantType  entities.TraceType
		wantValue string
	}{
		{name: "twitter", href: "https://twitter.com/acme", wantType: entities.Twitter, wantValue: "https://twitter.com/acme"},
		{name: "x", href: "https://x.com/acme", wantType: entities.Twitter, wantValue: "https://x.com/acme"},
		{name: "linkedin", href: "https://www.linkedin.com/in/jane-doe/", wantType: entities.Linkedin, wantValue: "https://www.linkedin.com/in/jane-doe"},
		{name: "github", href: "https://github.com/acme", wantType: entities.Github, wantValue: "https://github.com/acme"},
		{name: "facebook", href: "https://facebook.com/acme", wantType: entities.Facebook, wantValue: "https://facebook.com/acme"},
		{name: "instagram", href: "https://instagram.com/acme", wantType: entities.Instagram, wantValue: "https://instagram.com/acme"},
		{name: "tiktok", href: "https://tiktok.com/@acme", wantType: entities.TikTok, wantValue: "https://tiktok.com/@acme"},
		{name: "reddit", href: "https://reddit.com/u/acme", wantType: entities.Reddit, wantValue: "https://reddit.com/u/acme"},
		{name: "youtube", href: "https://youtube.com/channel/abc123", wantType: entities.YouTube, wantValue: "https://youtube.com/channel/abc123"},
		{name: "pinterest", href: "https://pinterest.com/acme", wantType: entities.Pinterest, wantValue: "https://pinterest.com/acme"},
		{name: "snapchat", href: "https://snapchat.com/add/acme", wantType: entities.Snapchat, wantValue: "https://snapchat.com/add/acme"},
		{name: "tumblr", href: "https://acme.tumblr.com", wantType: entities.Tumblr, wantValue: "https://acme.tumblr.com"},
		{name: "telegram", href: "https://t.me/acme", wantType: entities.SocialGeneric, wantValue: "https://t.me/acme"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace, ok := socialLinkTrace(tt.href)
			require.True(t, ok)
			assert.Equal(t, tt.wantType, trace.Type)
			assert.Equal(t, tt.wantValue, trace.Value)
		})
	}

	_, ok := socialLinkTrace("https://example.com/profile")
	assert.False(t, ok)
}
