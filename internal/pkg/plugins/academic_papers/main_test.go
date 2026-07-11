package academicpapers

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
		{"Username is followed", entities.Username, true},
		{"Name is followed", entities.Name, true},
		{"Domain is skipped", entities.Domain, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := &fakeSearchFetcher{responses: map[string]fakeResponse{}}
			p := &AcademicPapersPlugin{fetcher: fetcher}

			_, err := p.FollowTrace(entities.Trace{Value: "Jane Doe", Type: tt.traceType})
			require.NoError(t, err)
			assert.Equal(t, tt.wantCall, fetcher.lastURL != "")
		})
	}
}

func TestFollowTrace_ReturnsUrlTraces(t *testing.T) {
	body := `{"data":[{"title":"A Paper","url":"https://example.com/paper","authors":[{"name":"Jane Doe"}]}]}`
	fetcher := &fakeSearchFetcher{
		responses: map[string]fakeResponse{
			expectedSemanticScholarURL(t, "Jane Doe"): {status: http.StatusOK, body: body},
		},
	}
	p := &AcademicPapersPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(entities.Trace{Value: "Jane Doe", Type: entities.Username})
	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.Url, traces[0].Type)
	assert.Equal(t, "https://example.com/paper", traces[0].Value)
}

func TestRegister_RegistersUnderUsernameAndName(t *testing.T) {
	p := NewPlugin()
	require.NoError(t, p.Register())

	for _, traceType := range []entities.TraceType{entities.Username, entities.Name} {
		found := false
		for _, registered := range state.ActivePlugins[traceType] {
			if registered == p {
				found = true
			}
		}
		assert.Truef(t, found, "expected plugin registered for %v", traceType)
	}
}

func TestString(t *testing.T) {
	assert.Equal(t, "AcademicPapersPlugin", (&AcademicPapersPlugin{}).String())
}
