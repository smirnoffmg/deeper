package url_resolver

import (
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowTrace_WrongType(t *testing.T) {
	p := NewPlugin()

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_ExtractsHostAsDomain(t *testing.T) {
	p := NewPlugin()

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Url, Value: "https://codescoring.com/some/path?q=1"})
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.Domain, traces[0].Type)
	assert.Equal(t, "codescoring.com", traces[0].Value)
}

func TestFollowTrace_IPLiteralHostSkipped(t *testing.T) {
	p := NewPlugin()

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Url, Value: "http://192.168.1.1/admin"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_IPv6LiteralHostSkipped(t *testing.T) {
	p := NewPlugin()

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Url, Value: "http://[::1]/admin"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_MalformedURLReturnsNoTraces(t *testing.T) {
	p := NewPlugin()

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Url, Value: "://not-a-url"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestRegister_RegistersUnderUrl(t *testing.T) {
	p := NewPlugin()
	require.NoError(t, p.Register())

	found := false
	for _, registered := range state.ActivePlugins[entities.Url] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestString(t *testing.T) {
	assert.Equal(t, "URLResolverPlugin", NewPlugin().String())
}
