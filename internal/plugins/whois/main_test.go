package whois

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/stretchr/testify/assert"
)

func TestWhoisPlugin_FollowTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2?key=test-key&domain=example.com", r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		whoisInfo := WhoisInfo{
			Domain: "example.com",
		}
		json.NewEncoder(w).Encode(whoisInfo)
	}))
	defer server.Close()

	// Temporarily replace the fetchWhois function to use the mock server
	originalFetchWhois := fetchWhois
	fetchWhois = func(domain string, apiKey string) (*WhoisInfo, error) {
		url := server.URL + "/v2?key=" + apiKey + "&domain=" + domain
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var whoisInfo WhoisInfo
		if err := json.NewDecoder(resp.Body).Decode(&whoisInfo); err != nil {
			return nil, err
		}
		return &whoisInfo, nil
	}
	defer func() { fetchWhois = originalFetchWhois }()

	plugin := NewPlugin()
	trace := entities.Trace{
		Value: "example.com",
		Type:  entities.Domain,
	}

	t.Setenv("IP2WHOIS_API_KEY", "test-key")

	newTraces, err := plugin.FollowTrace(trace)
	assert.NoError(t, err)
	assert.Len(t, newTraces, 1)

	var whoisInfo WhoisInfo
	err = json.Unmarshal([]byte(newTraces[0].Value), &whoisInfo)
	assert.NoError(t, err)
	assert.Equal(t, "example.com", whoisInfo.Domain)
}
