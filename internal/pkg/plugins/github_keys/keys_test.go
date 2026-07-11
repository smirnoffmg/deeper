package github_keys

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResponse struct {
	status int
	body   string
}

type fakeKeyFetcher struct {
	responses map[string]fakeResponse
	lastURL   string
}

func (f *fakeKeyFetcher) Get(_ context.Context, url string) (*http.Response, error) {
	f.lastURL = url
	resp, ok := f.responses[url]
	if !ok {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("[]"))}, nil
	}
	return &http.Response{StatusCode: resp.status, Body: io.NopCloser(strings.NewReader(resp.body))}, nil
}

func sshURL(username string) string {
	return "https://api.github.com/users/" + username + "/keys"
}

func gpgURL(username string) string {
	return "https://api.github.com/users/" + username + "/gpg_keys"
}

func TestFetchSSHKeys_SingleKey(t *testing.T) {
	fetcher := &fakeKeyFetcher{
		responses: map[string]fakeResponse{
			sshURL("torvalds"): {status: http.StatusOK, body: `[{"id":1,"key":"ssh-rsa AAAAB3NzaC1yc2E torvalds@example.com","created_at":"2014-03-04T00:00:00Z"}]`},
		},
	}

	traces, err := fetchSSHKeys(context.Background(), fetcher, "torvalds")
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.SSHKey, traces[0].Type)
	assert.Equal(t, "ssh-rsa AAAAB3NzaC1yc2E torvalds@example.com", traces[0].Value)
}

func TestFetchSSHKeys_MultipleKeys(t *testing.T) {
	fetcher := &fakeKeyFetcher{
		responses: map[string]fakeResponse{
			sshURL("u"): {status: http.StatusOK, body: `[{"id":1,"key":"ssh-rsa AAA"},{"id":2,"key":"ssh-ed25519 BBB"}]`},
		},
	}

	traces, err := fetchSSHKeys(context.Background(), fetcher, "u")
	require.NoError(t, err)
	require.Len(t, traces, 2)
}

func TestFetchSSHKeys_EmptyList(t *testing.T) {
	fetcher := &fakeKeyFetcher{
		responses: map[string]fakeResponse{
			sshURL("u"): {status: http.StatusOK, body: `[]`},
		},
	}

	traces, err := fetchSSHKeys(context.Background(), fetcher, "u")
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFetchSSHKeys_NonOKStatusReturnsError(t *testing.T) {
	fetcher := &fakeKeyFetcher{
		responses: map[string]fakeResponse{
			sshURL("u"): {status: http.StatusNotFound, body: `{"message":"Not Found"}`},
		},
	}

	_, err := fetchSSHKeys(context.Background(), fetcher, "u")
	assert.Error(t, err)
}

func TestFetchGPGKeys_VerifiedEmailIncluded(t *testing.T) {
	fetcher := &fakeKeyFetcher{
		responses: map[string]fakeResponse{
			gpgURL("wycats"): {status: http.StatusOK, body: `[{
				"key_id": "058C8C4EB1B1A088",
				"emails": [{"email": "wycats@gmail.com", "verified": true}]
			}]`},
		},
	}

	traces, err := fetchGPGKeys(context.Background(), fetcher, "wycats")
	require.NoError(t, err)
	require.Len(t, traces, 2)

	types := map[entities.TraceType]string{}
	for _, tr := range traces {
		types[tr.Type] = tr.Value
	}
	assert.Equal(t, "058C8C4EB1B1A088", types[entities.PGPKey])
	assert.Equal(t, "wycats@gmail.com", types[entities.Email])
}

func TestFetchGPGKeys_UnverifiedEmailExcluded(t *testing.T) {
	fetcher := &fakeKeyFetcher{
		responses: map[string]fakeResponse{
			gpgURL("u"): {status: http.StatusOK, body: `[{
				"key_id": "ABC123",
				"emails": [{"email": "spoofed@example.com", "verified": false}]
			}]`},
		},
	}

	traces, err := fetchGPGKeys(context.Background(), fetcher, "u")
	require.NoError(t, err)

	for _, tr := range traces {
		assert.NotEqual(t, entities.Email, tr.Type)
	}
	require.Len(t, traces, 1)
	assert.Equal(t, entities.PGPKey, traces[0].Type)
}

func TestFetchGPGKeys_MultipleKeys(t *testing.T) {
	fetcher := &fakeKeyFetcher{
		responses: map[string]fakeResponse{
			gpgURL("u"): {status: http.StatusOK, body: `[
				{"key_id": "AAA", "emails": [{"email": "a@example.com", "verified": true}]},
				{"key_id": "BBB", "emails": [{"email": "b@example.com", "verified": true}]}
			]`},
		},
	}

	traces, err := fetchGPGKeys(context.Background(), fetcher, "u")
	require.NoError(t, err)
	assert.Len(t, traces, 4)
}

func TestFetchGPGKeys_NonOKStatusReturnsError(t *testing.T) {
	fetcher := &fakeKeyFetcher{
		responses: map[string]fakeResponse{
			gpgURL("u"): {status: http.StatusForbidden, body: `{"message":"rate limited"}`},
		},
	}

	_, err := fetchGPGKeys(context.Background(), fetcher, "u")
	assert.Error(t, err)
}

func TestFetchGPGKeys_MalformedJSONReturnsError(t *testing.T) {
	fetcher := &fakeKeyFetcher{
		responses: map[string]fakeResponse{
			gpgURL("u"): {status: http.StatusOK, body: `not json`},
		},
	}

	_, err := fetchGPGKeys(context.Background(), fetcher, "u")
	assert.Error(t, err)
}
