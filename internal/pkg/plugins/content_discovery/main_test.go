package content_discovery

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeFetcher struct {
	statuses map[string]int
	errs     map[string]error
	honorCtx bool
}

func (f *fakeFetcher) Get(ctx context.Context, url string) (*http.Response, error) {
	if f.honorCtx {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if err, ok := f.errs[url]; ok {
		return nil, err
	}
	status, ok := f.statuses[url]
	if !ok {
		status = http.StatusNotFound
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func newPlugin(f *fakeFetcher, words []string) *ContentDiscoveryPlugin {
	return &ContentDiscoveryPlugin{fetcher: f, words: words, concurrency: 4}
}

func urlTrace(v string) entities.Trace {
	return entities.Trace{Type: entities.Url, Value: v}
}

func TestFollowTrace_EmitsExistingPaths(t *testing.T) {
	f := &fakeFetcher{statuses: map[string]int{
		"https://example.com/admin":      http.StatusForbidden,
		"https://example.com/robots.txt": http.StatusOK,
		"https://example.com/missing":    http.StatusNotFound,
	}}
	p := newPlugin(f, []string{"admin", "robots.txt", "missing"})

	traces, err := p.FollowTrace(context.Background(), urlTrace("https://example.com/some/deep/path"))
	require.NoError(t, err)

	got := map[string]bool{}
	for _, tr := range traces {
		assert.Equal(t, entities.Url, tr.Type)
		got[tr.Value] = true
	}
	assert.Len(t, traces, 2)
	assert.True(t, got["https://example.com/admin"])
	assert.True(t, got["https://example.com/robots.txt"])
	assert.False(t, got["https://example.com/missing"])
}

func TestFollowTrace_AllFoundStatuses(t *testing.T) {
	f := &fakeFetcher{statuses: map[string]int{
		"https://h/a": http.StatusOK,
		"https://h/b": http.StatusNoContent,
		"https://h/c": http.StatusMovedPermanently,
		"https://h/d": http.StatusFound,
		"https://h/e": http.StatusTemporaryRedirect,
		"https://h/f": http.StatusUnauthorized,
		"https://h/g": http.StatusForbidden,
		"https://h/x": http.StatusInternalServerError,
		"https://h/y": http.StatusNotFound,
	}}
	p := newPlugin(f, []string{"a", "b", "c", "d", "e", "f", "g", "x", "y"})

	traces, err := p.FollowTrace(context.Background(), urlTrace("https://h"))
	require.NoError(t, err)
	assert.Len(t, traces, 7)
}

func TestFollowTrace_Deduplicates(t *testing.T) {
	f := &fakeFetcher{statuses: map[string]int{
		"https://example.com/admin": http.StatusOK,
	}}
	p := newPlugin(f, []string{"admin", "admin", "admin"})

	traces, err := p.FollowTrace(context.Background(), urlTrace("https://example.com"))
	require.NoError(t, err)
	assert.Len(t, traces, 1)
}

func TestFollowTrace_RequestErrorSkipped(t *testing.T) {
	f := &fakeFetcher{
		statuses: map[string]int{"https://example.com/ok": http.StatusOK},
		errs:     map[string]error{"https://example.com/boom": assert.AnError},
	}
	p := newPlugin(f, []string{"ok", "boom"})

	traces, err := p.FollowTrace(context.Background(), urlTrace("https://example.com"))
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, "https://example.com/ok", traces[0].Value)
}

func TestFollowTrace_WrongTraceType(t *testing.T) {
	p := newPlugin(&fakeFetcher{}, []string{"admin"})

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_UnparsableURLSkipped(t *testing.T) {
	p := newPlugin(&fakeFetcher{}, []string{"admin"})

	traces, err := p.FollowTrace(context.Background(), urlTrace("not a url"))
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_ContextCancelledReturnsPromptly(t *testing.T) {
	f := &fakeFetcher{honorCtx: true, statuses: map[string]int{
		"https://example.com/admin": http.StatusOK,
	}}
	p := newPlugin(f, []string{"admin", "login", "robots.txt"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		_, err := p.FollowTrace(ctx, urlTrace("https://example.com"))
		require.NoError(t, err)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("FollowTrace did not return promptly after context cancellation")
	}
}

func TestActive(t *testing.T) {
	assert.True(t, (&ContentDiscoveryPlugin{}).Active())
}

func TestRegister_RegistersUnderUrl(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	found := false
	for _, registered := range registry[entities.Url] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestNewPlugin_Defaults(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	assert.NotEmpty(t, p.words)
	assert.Positive(t, p.concurrency)
}

func TestString(t *testing.T) {
	assert.Equal(t, "ContentDiscoveryPlugin", (&ContentDiscoveryPlugin{}).String())
}
