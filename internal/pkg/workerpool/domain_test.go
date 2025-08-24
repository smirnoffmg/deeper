package workerpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDomainExtractor(t *testing.T) {
	extractor := NewDomainExtractor()
	require.NotNil(t, extractor)
	assert.NotNil(t, extractor.emailRegex)
	assert.NotNil(t, extractor.urlRegex)
}

func TestDomainExtractor_ExtractDomain(t *testing.T) {
	extractor := NewDomainExtractor()

	tests := []struct {
		name     string
		task     *Task
		expected string
		hasError bool
	}{
		{
			name: "extract domain from email",
			task: &Task{
				ID:      "test-email",
				Payload: "user@example.com",
			},
			expected: "example.com",
			hasError: false,
		},
		{
			name: "extract domain from URL",
			task: &Task{
				ID:      "test-url",
				Payload: "https://api.github.com/user/repos",
			},
			expected: "api.github.com",
			hasError: false,
		},
		{
			name: "extract domain from domain-only",
			task: &Task{
				ID:      "test-domain",
				Payload: "google.com",
			},
			expected: "google.com",
			hasError: false,
		},
		{
			name: "return default for non-domain content",
			task: &Task{
				ID:      "test-other",
				Payload: "some random text",
			},
			expected: "default",
			hasError: false,
		},
		{
			name:     "nil task",
			task:     nil,
			expected: "",
			hasError: true,
		},
		{
			name: "nil payload",
			task: &Task{
				ID:      "test-nil",
				Payload: nil,
			},
			expected: "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := extractor.ExtractDomain(tt.task)

			if tt.hasError {
				assert.Error(t, err)
				assert.Empty(t, domain)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, domain)
			}
		})
	}
}

func TestDomainExtractor_ExtractEmailDomain(t *testing.T) {
	extractor := NewDomainExtractor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid email",
			input:    "user@example.com",
			expected: "example.com",
		},
		{
			name:     "email with subdomain",
			input:    "user@api.github.com",
			expected: "api.github.com",
		},
		{
			name:     "invalid email",
			input:    "invalid-email",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.extractEmailDomain(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDomainExtractor_ExtractURLDomain(t *testing.T) {
	extractor := NewDomainExtractor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "https URL",
			input:    "https://api.github.com/user/repos",
			expected: "api.github.com",
		},
		{
			name:     "http URL",
			input:    "http://example.com/path",
			expected: "example.com",
		},
		{
			name:     "URL with port",
			input:    "https://localhost:8080/api",
			expected: "localhost:8080",
		},
		{
			name:     "invalid URL",
			input:    "not-a-url",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.extractURLDomain(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDomainExtractor_ExtractDomainOnly(t *testing.T) {
	extractor := NewDomainExtractor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid domain",
			input:    "example.com",
			expected: "example.com",
		},
		{
			name:     "domain with subdomain",
			input:    "api.github.com",
			expected: "api.github.com",
		},
		{
			name:     "domain with multiple subdomains",
			input:    "www.api.github.com",
			expected: "www.api.github.com",
		},
		{
			name:     "invalid domain",
			input:    "not-a-domain",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.extractDomainOnly(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDomainExtractor_ValidateDomain(t *testing.T) {
	extractor := NewDomainExtractor()

	tests := []struct {
		name     string
		domain   string
		expected bool
	}{
		{
			name:     "valid domain",
			domain:   "example.com",
			expected: true,
		},
		{
			name:     "domain with subdomain",
			domain:   "api.github.com",
			expected: true,
		},
		{
			name:     "default domain",
			domain:   "default",
			expected: true,
		},
		{
			name:     "empty domain",
			domain:   "",
			expected: true,
		},
		{
			name:     "invalid domain",
			domain:   "not-a-domain",
			expected: false,
		},
		{
			name:     "domain with invalid characters",
			domain:   "example@.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.ValidateDomain(tt.domain)
			assert.Equal(t, tt.expected, result)
		})
	}
}
