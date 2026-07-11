package companyregistry

import (
	"net/http"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowTrace_InputTypes(t *testing.T) {
	tests := []struct {
		name      string
		traceType entities.TraceType
		wantCall  bool
	}{
		{"Company is followed", entities.Company, true},
		{"Domain is skipped", entities.Domain, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := &fakeSearchFetcher{responses: map[string]fakeResponse{}}
			p := &CompanyRegistryPlugin{fetcher: fetcher}

			_, err := p.FollowTrace(entities.Trace{Value: "7813227385", Type: tt.traceType})
			require.NoError(t, err)
			assert.Equal(t, tt.wantCall, fetcher.lastURL != "")
		})
	}
}

func TestFollowTrace_ReturnsExtractedTraces(t *testing.T) {
	fetcher := &fakeSearchFetcher{
		responses: map[string]fakeResponse{
			expectedListOrgURL(t, "7813227385"): {status: http.StatusOK, body: realFixture},
		},
	}
	p := &CompanyRegistryPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(entities.Trace{Value: "7813227385", Type: entities.Company})
	require.NoError(t, err)
	assert.Len(t, traces, 3)
}

func TestRegister_RegistersUnderCompanyOnly(t *testing.T) {
	p := NewPlugin()
	require.NoError(t, p.Register())

	found := false
	for _, registered := range state.ActivePlugins[entities.Company] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestString(t *testing.T) {
	assert.Equal(t, "CompanyRegistryPlugin", (&CompanyRegistryPlugin{}).String())
}
