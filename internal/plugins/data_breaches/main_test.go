package data_breaches

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/stretchr/testify/assert"
)

func TestDataBreachesPlugin_FollowTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/breachedaccount/test@example.com", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		breaches := []Breach{
			{Name: "Breach1"},
			{Name: "Breach2"},
		}
		json.NewEncoder(w).Encode(breaches)
	}))
	defer server.Close()

	plugin := NewPlugin()
	trace := entities.Trace{
		Value: "test@example.com",
		Type:  entities.Email,
	}

	// Temporarily replace the hibpApiUrl to use the mock server
	originalApiUrl := hibpApiUrl
	hibpApiUrl = server.URL
	defer func() { hibpApiUrl = originalApiUrl }()

	t.Setenv("HIBP_API_KEY", "test-key")

	newTraces, err := plugin.FollowTrace(trace)
	assert.NoError(t, err)
	assert.Len(t, newTraces, 2)
	assert.Equal(t, "Breach1", newTraces[0].Value)
	assert.Equal(t, entities.DataBreach, newTraces[0].Type)
	assert.Equal(t, "Breach2", newTraces[1].Value)
	assert.Equal(t, entities.DataBreach, newTraces[1].Type)
}
