package namesake

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResponse struct {
	status int
	err    error
}

type fakeFetcher struct {
	responses map[string]fakeResponse
	reqs      []*http.Request
}

func (f *fakeFetcher) Do(req *http.Request) (*http.Response, error) {
	f.reqs = append(f.reqs, req)
	resp, ok := f.responses[req.URL.String()]
	if !ok {
		resp = fakeResponse{status: http.StatusNotFound}
	}
	if resp.err != nil {
		return nil, resp.err
	}
	return &http.Response{StatusCode: resp.status, Body: io.NopCloser(strings.NewReader("{}"))}, nil
}

const (
	githubProbe = "https://api.github.com/users/alice"
	habrProbe   = "https://habr.com/kek/v2/users/alice/card"
)

var (
	githubAccount = plugins.Finding{
		Trace:    entities.Trace{Type: entities.Github, Value: "https://github.com/alice"},
		Strength: entities.StrengthWeak,
		Basis:    "namesake: github.com/alice exists",
	}
	habrAccount = plugins.Finding{
		Trace:    entities.Trace{Type: entities.SocialGeneric, Value: "https://habr.com/ru/users/alice"},
		Strength: entities.StrengthWeak,
		Basis:    "namesake: habr.com/users/alice exists",
	}
)

func follow(t *testing.T, fetcher *fakeFetcher, value string) ([]plugins.Finding, error) {
	t.Helper()
	p := &NamesakePlugin{fetcher: fetcher}
	return p.FollowEvidence(context.Background(), entities.Trace{Type: entities.Username, Value: value})
}

func TestFollowEvidence_ExistingAccountsAreWeakFindings(t *testing.T) {
	findings, err := follow(t, &fakeFetcher{responses: map[string]fakeResponse{
		githubProbe: {status: http.StatusOK},
		habrProbe:   {status: http.StatusOK},
	}}, "alice")

	require.NoError(t, err)
	assert.ElementsMatch(t, []plugins.Finding{githubAccount, habrAccount}, findings)
}

func TestFollowEvidence_NotFoundIsNoFinding(t *testing.T) {
	findings, err := follow(t, &fakeFetcher{responses: map[string]fakeResponse{
		habrProbe: {status: http.StatusOK},
	}}, "alice")

	require.NoError(t, err)
	assert.Equal(t, []plugins.Finding{habrAccount}, findings)
}

func TestFollowEvidence_OnePlatformFailingKeepsTheOther(t *testing.T) {
	findings, err := follow(t, &fakeFetcher{responses: map[string]fakeResponse{
		githubProbe: {status: http.StatusOK},
		habrProbe:   {err: errors.New("connection reset")},
	}}, "alice")

	require.NoError(t, err)
	assert.Equal(t, []plugins.Finding{githubAccount}, findings)
}

func TestFollowEvidence_ErrorStatusIsNoFinding(t *testing.T) {
	findings, err := follow(t, &fakeFetcher{responses: map[string]fakeResponse{
		githubProbe: {status: http.StatusForbidden},
		habrProbe:   {status: http.StatusTooManyRequests},
	}}, "alice")

	assert.Error(t, err)
	assert.Empty(t, findings)
}

// Sherlock's data.json builds habr as https://habr.com/ru/users/{} (no
// trailing slash); any other shape splits one account into two nodes.
func TestFollowEvidence_HabrAccountMatchesSherlockShape(t *testing.T) {
	findings, err := follow(t, &fakeFetcher{responses: map[string]fakeResponse{habrProbe: {status: http.StatusOK}}}, "alice")

	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "https://habr.com/ru/users/alice", findings[0].Trace.Value)
}

// entities.NewTrace lowercases a github.com seed, so the namesake account
// must be lowercased too or the engine keeps two nodes for one account.
func TestFollowEvidence_GithubAccountIsLowercased(t *testing.T) {
	findings, err := follow(t, &fakeFetcher{responses: map[string]fakeResponse{
		"https://api.github.com/users/Alice": {status: http.StatusOK},
	}}, "Alice")

	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "https://github.com/alice", findings[0].Trace.Value)
}

func TestFollowEvidence_HandleUnusableInURLIsNotProbed(t *testing.T) {
	fetcher := &fakeFetcher{}
	findings, err := follow(t, fetcher, "alice/../admin")

	require.NoError(t, err)
	assert.Empty(t, findings)
	assert.Empty(t, fetcher.reqs)
}

func TestFollowEvidence_WrongTypeIsIgnored(t *testing.T) {
	fetcher := &fakeFetcher{}
	p := &NamesakePlugin{fetcher: fetcher}

	findings, err := p.FollowEvidence(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})

	require.NoError(t, err)
	assert.Empty(t, findings)
	assert.Empty(t, fetcher.reqs)
}

func TestFollowTrace_StripsEvidence(t *testing.T) {
	p := &NamesakePlugin{fetcher: &fakeFetcher{responses: map[string]fakeResponse{githubProbe: {status: http.StatusOK}}}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Username, Value: "alice"})

	require.NoError(t, err)
	assert.Equal(t, []entities.Trace{githubAccount.Trace}, traces)
}

func TestFollowEvidence_GitHubTokenSentOnlyToGitHub(t *testing.T) {
	fetcher := &fakeFetcher{}
	p := &NamesakePlugin{fetcher: fetcher, githubToken: "ghp_secret"}

	_, err := p.FollowEvidence(context.Background(), entities.Trace{Type: entities.Username, Value: "alice"})

	require.NoError(t, err)
	headers := map[string]string{}
	for _, req := range fetcher.reqs {
		headers[req.URL.String()] = req.Header.Get("Authorization")
	}
	require.Contains(t, headers, githubProbe)
	require.Contains(t, headers, habrProbe)
	assert.Equal(t, "Bearer ghp_secret", headers[githubProbe])
	assert.Empty(t, headers[habrProbe])
}

func TestRegister_RegistersUnderUsername(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	assert.Contains(t, registry[entities.Username], plugins.DeeperPlugin(p))
}

func TestString(t *testing.T) {
	assert.Equal(t, "NamesakePlugin", (&NamesakePlugin{}).String())
}
