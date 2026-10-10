package github_identity

import (
	"context"
	"net/http"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestIsFork_True(t *testing.T) {
	fetcher := &fakeCommitFetcher{status: http.StatusOK, body: `{"fork": true}`}
	assert.True(t, isFork(context.Background(), fetcher, "owner", "repo", ""))
}

func TestIsFork_False(t *testing.T) {
	fetcher := &fakeCommitFetcher{status: http.StatusOK, body: `{"fork": false}`}
	assert.False(t, isFork(context.Background(), fetcher, "owner", "repo", ""))
}

func TestIsFork_MalformedJSONTreatedAsNotFork(t *testing.T) {
	fetcher := &fakeCommitFetcher{status: http.StatusOK, body: `not json`}
	assert.False(t, isFork(context.Background(), fetcher, "owner", "repo", ""))
}

func TestIsFork_NonOKStatusTreatedAsNotFork(t *testing.T) {
	fetcher := &fakeCommitFetcher{status: http.StatusNotFound, body: `{"message":"Not Found"}`}
	assert.False(t, isFork(context.Background(), fetcher, "owner", "repo", ""))
}

func TestParseCommitAuthors_CommitterDiffersFromAuthor(t *testing.T) {
	body := []byte(`[{
		"author": {"login": "alsmirn"},
		"commit": {
			"author": {"name": "Alexey Smirnov", "email": "alsmirn@gmail.com"},
			"committer": {"name": "Someone Else", "email": "someone@example.com"},
			"message": "a commit"
		}
	}]`)

	authors := parseCommitAuthors(body)

	require.Len(t, authors, 2)
	assert.Equal(t, "Alexey Smirnov", authors[0].Name)
	assert.Equal(t, "alsmirn@gmail.com", authors[0].Email)
	assert.Equal(t, "alsmirn", authors[0].Login)
	assert.Equal(t, "Someone Else", authors[1].Name)
	assert.Equal(t, "someone@example.com", authors[1].Email)
	assert.Empty(t, authors[1].Login)
}

func TestParseCommitAuthors_IdenticalCommitterIsStillEmitted(t *testing.T) {
	// authorsToFindings already dedupes downstream by type+value, so
	// parseCommitAuthors doesn't need its own identity check here.
	body := []byte(`[{
		"commit": {
			"author": {"name": "Alexey Smirnov", "email": "alsmirn@gmail.com"},
			"committer": {"name": "Alexey Smirnov", "email": "alsmirn@gmail.com"},
			"message": "a commit"
		}
	}]`)

	authors := parseCommitAuthors(body)

	require.Len(t, authors, 2)
	assert.Equal(t, authors[0].Name, authors[1].Name)
	assert.Equal(t, authors[0].Email, authors[1].Email)
}

func TestParseCommitAuthors_CoAuthorTrailer(t *testing.T) {
	body := []byte(`[{
		"commit": {
			"author": {"name": "Alexey Smirnov", "email": "alsmirn@gmail.com"},
			"committer": {"name": "Alexey Smirnov", "email": "alsmirn@gmail.com"},
			"message": "Fix bug\n\nCo-authored-by: Serge Matveenko <s@matveenko.ru>"
		}
	}]`)

	authors := parseCommitAuthors(body)

	require.Len(t, authors, 3)
	assert.Equal(t, "Serge Matveenko", authors[2].Name)
	assert.Equal(t, "s@matveenko.ru", authors[2].Email)
	assert.Empty(t, authors[2].Login)
}

func TestParseCoAuthors(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    []commitAuthor
	}{
		{
			name:    "single trailer",
			message: "Fix bug\n\nCo-authored-by: Serge Matveenko <s@matveenko.ru>",
			want:    []commitAuthor{{Name: "Serge Matveenko", Email: "s@matveenko.ru", CoAuthor: true}},
		},
		{
			name: "multiple trailers",
			message: "Fix bug\n\nCo-authored-by: A One <a@example.com>\n" +
				"Co-authored-by: B Two <b@example.com>",
			want: []commitAuthor{
				{Name: "A One", Email: "a@example.com", CoAuthor: true},
				{Name: "B Two", Email: "b@example.com", CoAuthor: true},
			},
		},
		{
			name:    "case-insensitive prefix",
			message: "Fix bug\n\nco-authored-by: A One <a@example.com>",
			want:    []commitAuthor{{Name: "A One", Email: "a@example.com", CoAuthor: true}},
		},
		{
			name:    "no trailer",
			message: "Just a normal commit message",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseCoAuthors(tt.message))
		})
	}
}

func TestDistinctIdentityCount(t *testing.T) {
	authors := []commitAuthor{
		{Login: "alsmirn", Name: "Alexey Smirnov", Email: "alsmirn@gmail.com"},
		{Login: "alsmirn", Name: "Alexey Smirnov", Email: "alsmirn@gmail.com"}, // duplicate
		{Name: "Serge Matveenko", Email: "s@matveenko.ru"},                     // no login
		{Name: "Someone Else"}, // no login, no email
	}
	assert.Equal(t, 3, distinctIdentityCount(authors))
}

func TestDistinctIdentityCount_EmptyEntriesIgnored(t *testing.T) {
	assert.Equal(t, 0, distinctIdentityCount([]commitAuthor{{}}))
}

func TestFilterAuthorsForSharedRepo_SmallRepoKeepsEveryone(t *testing.T) {
	authors := []commitAuthor{
		{Login: "alsmirn", Name: "Alexey Smirnov"},
		{Name: "Serge Matveenko", Email: "s@matveenko.ru"},
	}

	filtered := filterAuthorsForSharedRepo(authors, "alsmirn")
	assert.Equal(t, authors, filtered)
}

// TestFilterAuthorsForSharedRepo_LargeRepoKeepsOnlyOwner is a regression
// test found live against codescoring.ru: iloncka/workshop-astra-tik-tok
// (not a fork — isFork correctly reports false) turned out to have commits
// from several unrelated DataStax employees, leaking their names/emails
// into the graph purely because iloncka once contributed to a shared
// workshop repo. A repo with many distinct contributors is unlikely to
// represent personal connections; only the repo owner's own commits should
// be trusted in that case.
func TestFilterAuthorsForSharedRepo_LargeRepoKeepsOnlyOwner(t *testing.T) {
	authors := []commitAuthor{
		{Login: "iloncka", Name: "Ilona Kovaleva"},
		{Name: "Erick Ramirez", Email: "erick@datastax.com"},
		{Name: "Stefano Lottini", Email: "stefano@datastax.com"},
		{Name: "Cedrick Lunven", Email: "cedrick@datastax.com"},
		{Name: "David Jones-Gilardi", Email: "david@datastax.com"},
		{Name: "Kirsten Hunter", Email: "kirsten@datastax.com"},
	}

	filtered := filterAuthorsForSharedRepo(authors, "iloncka")
	require.Len(t, filtered, 1)
	assert.Equal(t, "iloncka", filtered[0].Login)
}

func TestFilterAuthorsForSharedRepo_OwnerMatchIsCaseInsensitive(t *testing.T) {
	authors := []commitAuthor{
		{Login: "Iloncka", Name: "Ilona Kovaleva"},
		{Name: "A", Email: "a@x.com"}, {Name: "B", Email: "b@x.com"},
		{Name: "C", Email: "c@x.com"}, {Name: "D", Email: "d@x.com"},
		{Name: "E", Email: "e@x.com"},
	}

	filtered := filterAuthorsForSharedRepo(authors, "iloncka")
	require.Len(t, filtered, 1)
	assert.Equal(t, "Iloncka", filtered[0].Login)
}

func TestAuthorsToFindings(t *testing.T) {
	authors := []commitAuthor{
		{Name: "Alexey Smirnov", Email: "alex@smirnov.dev", Login: "alsmirn"},
		{Name: "Alexey Smirnov", Email: "alex@smirnov.dev", Login: "alsmirn"},
		{Name: "Bot User", Email: "bot@ci.dev"},
	}

	findings := authorsToFindings(authors)

	names, emails, usernames, githubs := 0, 0, 0, 0
	for _, f := range findings {
		tr := f.Trace
		switch tr.Type {
		case entities.Name:
			names++
		case entities.Email:
			emails++
		case entities.Username:
			usernames++
		case entities.Github:
			githubs++
			assert.Equal(t, "https://github.com/alsmirn", tr.Value)
		}
	}

	assert.Equal(t, 2, names)
	assert.Equal(t, 2, emails)
	assert.Equal(t, 1, usernames)
	assert.Equal(t, 1, githubs)
}

// Regression: git falls back to placeholder addresses like "you@example.com"
// when a contributor never configures user.email. These are syntactically
// valid emails but never real contact info, so they must not become traces.
func TestAuthorsToFindings_PlaceholderEmailRejected(t *testing.T) {
	authors := []commitAuthor{
		{Name: "Someone", Email: "you@example.com", Login: "someone"},
	}

	findings := authorsToFindings(authors)

	for _, f := range findings {
		tr := f.Trace
		assert.NotEqual(t, entities.Email, tr.Type, "placeholder email must not become a trace")
	}
}

func TestAuthorsToFindings_MalformedEmailRejected(t *testing.T) {
	authors := []commitAuthor{
		{Name: "Someone", Email: "not-an-email", Login: "someone"},
	}

	findings := authorsToFindings(authors)

	for _, f := range findings {
		tr := f.Trace
		assert.NotEqual(t, entities.Email, tr.Type, "malformed email must not become a trace")
	}
}

func TestParseCommitAuthors_DropsWebFlowCommitter(t *testing.T) {
	body := []byte(`[{
		"author": {"login": "smirnoffmg"},
		"commit": {
			"author": {"name": "Maksim Smirnov", "email": "smirnoffmg@gmail.com"},
			"committer": {"name": "GitHub", "email": "noreply@github.com"},
			"message": "Merge pull request #1"
		}
	}]`)

	authors := parseCommitAuthors(body)

	require.Len(t, authors, 1)
	assert.Equal(t, "Maksim Smirnov", authors[0].Name)
}

func TestParseCommitAuthors_DropsBotIdentities(t *testing.T) {
	body := []byte(`[
		{
			"author": {"login": "dependabot[bot]"},
			"commit": {
				"author": {"name": "dependabot[bot]", "email": "49699333+dependabot[bot]@users.noreply.github.com"},
				"committer": {"name": "GitHub", "email": "noreply@github.com"},
				"message": "Bump x\n\nCo-authored-by: github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>"
			}
		},
		{
			"commit": {
				"author": {"name": "Renovate Bot", "email": "29139614+renovate[bot]@users.noreply.github.com"},
				"committer": {"name": "github-actions[bot]", "email": "actions@example.org"},
				"message": "chore"
			}
		}
	]`)

	assert.Empty(t, parseCommitAuthors(body))
}

func TestParseCommitAuthors_DropsAutomationWithoutBotMarker(t *testing.T) {
	body := []byte(`[
		{
			"author": {"login": "Copilot", "type": "Bot"},
			"committer": {"login": "Copilot", "type": "Bot"},
			"commit": {
				"author": {"name": "Copilot", "email": "198982749+Copilot@users.noreply.github.com"},
				"committer": {"name": "Copilot", "email": "198982749+Copilot@users.noreply.github.com"},
				"message": "Fix tests"
			}
		},
		{
			"author": {"login": "smirnoffmg", "type": "User"},
			"commit": {
				"author": {"name": "Maksim Smirnov", "email": "smirnoffmg@gmail.com"},
				"committer": {"name": "GitHub Action", "email": "action@github.com"},
				"message": "Update\n\nCo-authored-by: Copilot <175728472+Copilot@users.noreply.github.com>"
			}
		},
		{
			"commit": {
				"author": {"name": "github-actions", "email": "github-actions@github.com"},
				"committer": {"name": "github-actions", "email": "github-actions@github.com"},
				"message": "Release"
			}
		}
	]`)

	authors := parseCommitAuthors(body)

	require.Len(t, authors, 1)
	assert.Equal(t, "Maksim Smirnov", authors[0].Name)
}

func TestParseCommitAuthors_BotsDoNotMakeRepoShared(t *testing.T) {
	commit := func(login string) string {
		return `{"author": {"login": "` + login + `[bot]"}, "commit": {"author": {"name": "` + login + `[bot]", "email": "1+` + login + `[bot]@users.noreply.github.com"}, "committer": {"name": "GitHub", "email": "noreply@github.com"}, "message": "bump"}}`
	}
	body := []byte(`[
		{"author": {"login": "alice"}, "commit": {"author": {"name": "Alice", "email": "alice@example.com"}, "committer": {"name": "Bob", "email": "bob@example.com"}, "message": "init"}},
		` + commit("dependabot") + `, ` + commit("renovate") + `, ` + commit("github-actions") + `,
		` + commit("pre-commit-ci") + `, ` + commit("codecov") + `
	]`)

	authors := filterAuthorsForSharedRepo(parseCommitAuthors(body), "alice")

	assert.ElementsMatch(t, []commitAuthor{
		{Name: "Alice", Email: "alice@example.com", Login: "alice"},
		{Name: "Bob", Email: "bob@example.com"},
	}, authors)
}

func TestParseCommitAuthors_CoAuthorIsMarked(t *testing.T) {
	body := []byte(`[{
		"author": {"login": "smirnoffmg"},
		"commit": {
			"author": {"name": "Maksim Smirnov", "email": "smirnoffmg@gmail.com"},
			"message": "feat\n\nCo-authored-by: Claude Opus 4.8 <noreply@anthropic.com>"
		}
	}]`)

	authors := parseCommitAuthors(body)

	require.Len(t, authors, 2)
	assert.False(t, authors[0].CoAuthor)
	assert.True(t, authors[1].CoAuthor)
}

func TestAuthorsToFindings_CoAuthorWeakAuthorStrong(t *testing.T) {
	authors := []commitAuthor{
		{Name: "Maksim Smirnov", Email: "smirnoffmg@gmail.com", Login: "smirnoffmg"},
		{Name: "Cursor", Email: "cursoragent@cursor.com", CoAuthor: true},
	}

	findings := authorsToFindings(authors)

	byValue := make(map[string]plugins.Finding)
	for _, f := range findings {
		byValue[f.Trace.Value] = f
	}
	require.Len(t, byValue, 6)
	for _, v := range []string{"Maksim Smirnov", "smirnoffmg@gmail.com", "smirnoffmg"} {
		assert.Equal(t, entities.StrengthStrong, byValue[v].Strength, v)
		assert.Equal(t, "commit author", byValue[v].Basis, v)
	}
	account := byValue["https://github.com/smirnoffmg"]
	assert.Equal(t, entities.Github, account.Trace.Type)
	assert.Equal(t, entities.StrengthStrong, account.Strength)
	assert.Equal(t, "commit author login", account.Basis)
	for _, v := range []string{"Cursor", "cursoragent@cursor.com"} {
		assert.Equal(t, entities.StrengthWeak, byValue[v].Strength, v)
		assert.Equal(t, "co-authored-by trailer", byValue[v].Basis, v)
	}
}

func TestAuthorsToFindings_AuthorSeenAfterCoAuthorIsStrong(t *testing.T) {
	authors := []commitAuthor{
		{Name: "Serge Matveenko", Email: "s@matveenko.ru", CoAuthor: true},
		{Name: "Serge Matveenko", Email: "s@matveenko.ru"},
	}

	findings := authorsToFindings(authors)

	require.Len(t, findings, 2)
	for _, f := range findings {
		assert.Equal(t, entities.StrengthStrong, f.Strength, f.Trace.Value)
	}
}

// The account node must match the lowercased form entities.NewTrace gives a
// github.com seed, or the engine keeps two nodes for one account.
func TestAuthorsToFindings_LoginAccountIsLowercased(t *testing.T) {
	findings := authorsToFindings([]commitAuthor{{Name: "Maksim", Login: "SmirnoffMG"}})

	var accounts []string
	for _, f := range findings {
		if f.Trace.Type == entities.Github {
			accounts = append(accounts, f.Trace.Value)
		}
	}
	assert.Equal(t, []string{"https://github.com/smirnoffmg"}, accounts)
}
