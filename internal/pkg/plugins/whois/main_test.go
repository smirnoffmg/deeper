package whois

import (
	"context"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowTrace_InputTypes(t *testing.T) {
	tests := []struct {
		name      string
		traceType entities.TraceType
		wantCall  bool
	}{
		{"Domain is followed", entities.Domain, true},
		{"Subdomain is skipped", entities.Subdomain, false},
		{"Username is skipped", entities.Username, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeWhoisClient{responses: map[string]string{}}
			p := &WhoisPlugin{client: client}

			_, err := p.FollowTrace(context.Background(), entities.Trace{Value: "example.ru", Type: tt.traceType})
			require.NoError(t, err)
			assert.Equal(t, tt.wantCall, client.lastQueried())
		})
	}
}

func TestRegister_RegistersUnderDomainOnly(t *testing.T) {
	p := NewPlugin()
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	found := false
	for _, registered := range registry[entities.Domain] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)

	for _, registered := range registry[entities.Subdomain] {
		assert.NotEqual(t, p, registered)
	}
}

func TestString(t *testing.T) {
	assert.Equal(t, "WhoisPlugin", (&WhoisPlugin{}).String())
}
