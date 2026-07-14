package github_keys

import (
	"net/http"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowTrace_WrongType(t *testing.T) {
	p := &GitHubKeysPlugin{fetcher: &fakeKeyFetcher{}}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Domain, Value: "example.com"})
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

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Username, Value: "alsmirn"})
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

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Username, Value: "u"})
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

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Username, Value: "u"})
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.SSHKey, traces[0].Type)
}

func TestRegister_RegistersUnderUsername(t *testing.T) {
	p := NewPlugin()
	require.NoError(t, p.Register())

	found := false
	for _, registered := range state.ActivePlugins[entities.Username] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestString(t *testing.T) {
	assert.Equal(t, "GitHubKeysPlugin", (&GitHubKeysPlugin{}).String())
}
