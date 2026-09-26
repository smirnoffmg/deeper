package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

func TestNew_RegistersEachPluginOncePerTraceType(t *testing.T) {
	// The error is ignored on purpose: social_profiles needs the network to
	// register, and every other plugin must register regardless.
	registry, _ := New(config.DefaultConfig())

	names := map[string]bool{}
	for traceType, ps := range registry {
		seen := map[string]bool{}
		for _, p := range ps {
			assert.Falsef(t, seen[p.String()], "%s registered twice for %s", p, traceType)
			seen[p.String()] = true
			names[p.String()] = true
		}
	}
	assert.GreaterOrEqual(t, len(names), 24)

	for _, tt := range []entities.TraceType{entities.Username, entities.Domain, entities.Email, entities.SocialGeneric, entities.IpAddr, entities.Url} {
		assert.NotEmptyf(t, registry[tt], "no plugin follows %s", tt)
	}
}
