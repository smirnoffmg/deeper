package habr_profile

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
	p := &HabrProfilePlugin{fetcher: &fakeProfileFetcher{}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_HabrAccount(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			cardURL("alsmirn"): {status: http.StatusOK, body: realFixture},
		},
	}
	p := &HabrProfilePlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.SocialGeneric, Value: "https://habr.com/ru/users/alsmirn/"})
	require.NoError(t, err)
	require.NotEmpty(t, traces)
}

func TestRegister_RegistersUnderSocialGeneric(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	found := false
	for _, registered := range registry[entities.SocialGeneric] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestMatches(t *testing.T) {
	p := &HabrProfilePlugin{}
	tests := []struct {
		trace entities.Trace
		want  bool
	}{
		{entities.Trace{Type: entities.SocialGeneric, Value: "https://habr.com/ru/users/alsmirn/"}, true},
		{entities.Trace{Type: entities.SocialGeneric, Value: "https://habr.com/en/users/alsmirn"}, true},
		{entities.Trace{Type: entities.SocialGeneric, Value: "https://habr.com/users/alsmirn"}, true},
		{entities.Trace{Type: entities.SocialGeneric, Value: "https://habr.com/ru/companies/codescoring/"}, false},
		{entities.Trace{Type: entities.SocialGeneric, Value: "https://habr.com/ru/users/"}, false},
		{entities.Trace{Type: entities.SocialGeneric, Value: "https://habr.com/ru/users/alsmirn/posts/"}, false},
		{entities.Trace{Type: entities.SocialGeneric, Value: "https://nothabr.com/ru/users/alsmirn/"}, false},
		{entities.Trace{Type: entities.SocialGeneric, Value: "https://t.me/alsmirn"}, false},
		{entities.Trace{Type: entities.Username, Value: "alsmirn"}, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, p.Matches(tt.trace), tt.trace.String())
	}
}

func TestFollowTrace_FetchesCardOfURLHandle(t *testing.T) {
	fetcher := &fakeProfileFetcher{}
	p := &HabrProfilePlugin{fetcher: fetcher}

	_, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.SocialGeneric, Value: "https://habr.com/ru/users/alsmirn/"})

	require.NoError(t, err)
	assert.Equal(t, cardURL("alsmirn"), fetcher.lastURL)
}

func TestFollowTrace_NonHabrAccountIsNotFetched(t *testing.T) {
	fetcher := &fakeProfileFetcher{}
	p := &HabrProfilePlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.SocialGeneric, Value: "https://t.me/alsmirn"})

	require.NoError(t, err)
	assert.Nil(t, traces)
	assert.Empty(t, fetcher.lastURL)
}

func TestString(t *testing.T) {
	assert.Equal(t, "HabrProfilePlugin", (&HabrProfilePlugin{}).String())
}
