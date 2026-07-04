package github_identity

import (
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
)

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		{
			name:      "valid repo URL",
			url:       "https://github.com/CodeScoring/awesome-open-source-licensing",
			wantOwner: "CodeScoring",
			wantRepo:  "awesome-open-source-licensing",
			wantOK:    true,
		},
		{
			name:      "trailing slash",
			url:       "https://github.com/owner/repo/",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantOK:    true,
		},
		{
			name:   "org root rejected",
			url:    "https://github.com/acme",
			wantOK: false,
		},
		{
			name:   "user profile rejected",
			url:    "https://github.com/johndoe",
			wantOK: false,
		},
		{
			name:   "malformed URL",
			url:    "not-a-url",
			wantOK: false,
		},
		{
			name:   "non-github host",
			url:    "https://gitlab.com/owner/repo",
			wantOK: false,
		},
		{
			name:   "extra path segments rejected",
			url:    "https://github.com/owner/repo/tree/main",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, ok := parseOwnerRepo(tt.url)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantOwner, owner)
				assert.Equal(t, tt.wantRepo, repo)
			}
		})
	}
}

func TestAuthorsToTraces(t *testing.T) {
	authors := []commitAuthor{
		{Name: "Alexey Smirnov", Email: "alex@example.com", Login: "alsmirn"},
		{Name: "Alexey Smirnov", Email: "alex@example.com", Login: "alsmirn"},
		{Name: "Bot User", Email: "bot@example.com"},
	}

	traces := authorsToTraces(authors)

	names, emails, usernames := 0, 0, 0
	for _, tr := range traces {
		switch tr.Type {
		case entities.Name:
			names++
		case entities.Email:
			emails++
		case entities.Username:
			usernames++
		}
	}

	assert.Equal(t, 2, names)
	assert.Equal(t, 2, emails)
	assert.Equal(t, 1, usernames)
}
