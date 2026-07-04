package crtsh

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCertFetcher struct {
	body       string
	statusCode int
	err        error
}

func (f *fakeCertFetcher) Get(url string) (*http.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	status := f.statusCode
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Header:     make(http.Header),
	}, nil
}

func TestSubdomainPlugin_FollowTrace_WrongType(t *testing.T) {
	plugin := &SubdomainPlugin{fetcher: &fakeCertFetcher{}}
	traces, err := plugin.FollowTrace(entities.Trace{Value: "x", Type: entities.Email})
	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestSubdomainPlugin_FollowTrace_ParsesJSON(t *testing.T) {
	plugin := &SubdomainPlugin{fetcher: &fakeCertFetcher{
		body: `[{"name_value":"www.example.com\napi.example.com"}]`,
	}}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "example.com", Type: entities.Domain})

	require.NoError(t, err)
	require.Len(t, traces, 2)
}

func TestSubdomainPlugin_FollowTrace_HTMLResponse(t *testing.T) {
	plugin := &SubdomainPlugin{fetcher: &fakeCertFetcher{
		body: `<html><body>rate limited</body></html>`,
	}}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "example.com", Type: entities.Domain})

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestSubdomainPlugin_FollowTrace_RequestError(t *testing.T) {
	plugin := &SubdomainPlugin{fetcher: &fakeCertFetcher{
		err: assert.AnError,
	}}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "example.com", Type: entities.Domain})

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestDecodeEntries_InvalidJSON(t *testing.T) {
	entries, ok := decodeEntries([]byte(`not json`), "example.com")
	assert.False(t, ok)
	assert.Nil(t, entries)
}
