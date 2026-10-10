package github_keys

import (
	"context"
	"net/http"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowTrace_WrongType(t *testing.T) {
	p := &GitHubKeysPlugin{fetcher: &fakeKeyFetcher{}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_MergesSSHAndGPGTraces(t *testing.T) {
	fetcher := &fakeKeyFetcher{
		responses: map[string]fakeResponse{
			sshURL("alsmirn"): {status: http.StatusOK, body: `[{"id":1,"key":"ssh-rsa AAA"}]`},
			gpgURL("alsmirn"): {status: http.StatusOK, body: `[{"key_id":"ABC","emails":[{"email":"a@keyholder.dev","verified":true}]}]`},
		},
	}
	p := &GitHubKeysPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Github, Value: "https://github.com/alsmirn"})
	require.NoError(t, err)
	require.Len(t, traces, 3)
}

func TestFollowTrace_SSHFailureDoesNotBlockGPG(t *testing.T) {
	fetcher := &fakeKeyFetcher{
		responses: map[string]fakeResponse{
			sshURL("u"): {status: http.StatusForbidden, body: `{"message":"nope"}`},
			gpgURL("u"): {status: http.StatusOK, body: `[{"key_id":"ABC","emails":[]}]`},
		},
	}
	p := &GitHubKeysPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Github, Value: "https://github.com/u"})
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.PGPKey, traces[0].Type)
}

func TestFollowTrace_GPGFailureDoesNotBlockSSH(t *testing.T) {
	fetcher := &fakeKeyFetcher{
		responses: map[string]fakeResponse{
			sshURL("u"): {status: http.StatusOK, body: `[{"id":1,"key":"ssh-rsa AAA"}]`},
			gpgURL("u"): {status: http.StatusForbidden, body: `{"message":"nope"}`},
		},
	}
	p := &GitHubKeysPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Github, Value: "https://github.com/u"})
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.SSHKey, traces[0].Type)
}

func TestFollowTrace_SetsAuthorizationWhenTokenPresent(t *testing.T) {
	fetcher := &fakeKeyFetcher{}
	p := &GitHubKeysPlugin{fetcher: fetcher, token: "ghp_secret"}

	_, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Github, Value: "https://github.com/alsmirn"})
	require.NoError(t, err)
	require.NotEmpty(t, fetcher.reqs)
	for _, req := range fetcher.reqs {
		assert.Equal(t, "Bearer ghp_secret", req.Header.Get("Authorization"), req.URL.String())
	}
}

func TestFollowTrace_NoAuthorizationWhenTokenAbsent(t *testing.T) {
	fetcher := &fakeKeyFetcher{}
	p := &GitHubKeysPlugin{fetcher: fetcher}

	_, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Github, Value: "https://github.com/alsmirn"})
	require.NoError(t, err)
	require.NotEmpty(t, fetcher.reqs)
	for _, req := range fetcher.reqs {
		assert.Empty(t, req.Header.Get("Authorization"), req.URL.String())
	}
}

func TestRegister_RegistersUnderGithub(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	found := false
	for _, registered := range registry[entities.Github] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestFollowTrace_IgnoresNonAccountTraces(t *testing.T) {
	for _, trace := range []entities.Trace{
		{Type: entities.Username, Value: "alsmirn"},
		{Type: entities.Github, Value: "https://github.com/alsmirn/repo"},
		{Type: entities.Github, Value: "https://gitlab.com/alsmirn"},
		{Type: entities.Github, Value: "https://github.com/"},
	} {
		fetcher := &fakeKeyFetcher{}
		p := &GitHubKeysPlugin{fetcher: fetcher}

		traces, err := p.FollowTrace(context.Background(), trace)

		require.NoError(t, err, trace.Value)
		assert.Nil(t, traces, trace.Value)
		assert.Empty(t, fetcher.reqs, trace.Value)
		assert.False(t, p.Matches(trace), trace.Value)
	}
}

func TestFollowTrace_AcceptsAccountURLShapes(t *testing.T) {
	for _, value := range []string{"https://github.com/alsmirn", "https://www.github.com/alsmirn/", "http://GitHub.com/alsmirn"} {
		fetcher := &fakeKeyFetcher{}
		p := &GitHubKeysPlugin{fetcher: fetcher}
		trace := entities.Trace{Type: entities.Github, Value: value}

		_, err := p.FollowTrace(context.Background(), trace)

		require.NoError(t, err, value)
		require.NotEmpty(t, fetcher.reqs, value)
		assert.Contains(t, fetcher.reqs[0].URL.Path, "/users/alsmirn", value)
		assert.True(t, p.Matches(trace), value)
	}
}

func TestString(t *testing.T) {
	assert.Equal(t, "GitHubKeysPlugin", (&GitHubKeysPlugin{}).String())
}
