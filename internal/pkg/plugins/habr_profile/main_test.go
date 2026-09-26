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

func TestFollowTrace_ValidUsername(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			cardURL("alsmirn"): {status: http.StatusOK, body: realFixture},
		},
	}
	p := &HabrProfilePlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Username, Value: "alsmirn"})
	require.NoError(t, err)
	require.NotEmpty(t, traces)
}

func TestRegister_RegistersUnderUsername(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	found := false
	for _, registered := range registry[entities.Username] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestString(t *testing.T) {
	assert.Equal(t, "HabrProfilePlugin", (&HabrProfilePlugin{}).String())
}
