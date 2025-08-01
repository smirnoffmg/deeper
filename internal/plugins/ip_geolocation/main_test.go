package ip_geolocation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/stretchr/testify/assert"
)

func TestIpGeolocationPlugin_FollowTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/json/8.8.8.8", r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		geolocationInfo := GeolocationInfo{
			Status: "success",
			Query:  "8.8.8.8",
		}
		json.NewEncoder(w).Encode(geolocationInfo)
	}))
	defer server.Close()

	// Temporarily replace the fetchGeolocation function to use the mock server
	originalFetchGeolocation := fetchGeolocation
	fetchGeolocation = func(ip string) (*GeolocationInfo, error) {
		url := server.URL + "/json/" + ip
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var geolocationInfo GeolocationInfo
		if err := json.NewDecoder(resp.Body).Decode(&geolocationInfo); err != nil {
			return nil, err
		}
		return &geolocationInfo, nil
	}
	defer func() { fetchGeolocation = originalFetchGeolocation }()

	plugin := NewPlugin()
	trace := entities.Trace{
		Value: "8.8.8.8",
		Type:  entities.IpAddr,
	}

	newTraces, err := plugin.FollowTrace(trace)
	assert.NoError(t, err)
	assert.Len(t, newTraces, 1)

	var geolocationInfo GeolocationInfo
	err = json.Unmarshal([]byte(newTraces[0].Value), &geolocationInfo)
	assert.NoError(t, err)
	assert.Equal(t, "8.8.8.8", geolocationInfo.Query)
}
