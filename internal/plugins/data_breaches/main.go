package data_breaches

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/state"
)

var (
	hibpApiUrl = "https://haveibeenpwned.com/api/v3"
)

const (
	InputTraceType = entities.Email
)

func init() {
	p := NewPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type DataBreachesPlugin struct{}

func NewPlugin() *DataBreachesPlugin {
	return &DataBreachesPlugin{}
}

func (p *DataBreachesPlugin) Register() error {
	state.RegisterPlugin(InputTraceType, p)
	return nil
}

type Breach struct {
	Name string `json:"Name"`
}

func (p *DataBreachesPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	var newTraces []entities.Trace

	breaches, err := fetchBreaches(trace.Value, hibpApiUrl)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to fetch breaches for %s", trace.Value)
		return nil, err
	}

	for _, breach := range breaches {
		newTraces = append(newTraces, entities.Trace{
			Value: breach.Name,
			Type:  entities.DataBreach,
		})
	}

	return newTraces, nil
}

func fetchBreaches(email string, apiUrl string) ([]Breach, error) {
	url := fmt.Sprintf("%s/breachedaccount/%s", apiUrl, email)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	apiKey := os.Getenv("HIBP_API_KEY")
	if apiKey == "" {
		log.Warn().Msg("HIBP_API_KEY environment variable not set. Skipping data breach check.")
		return nil, nil
	}
	req.Header.Set("hibp-api-key", apiKey)
	req.Header.Set("User-Agent", "deeper")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // No breaches found
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var breaches []Breach
	if err := json.NewDecoder(resp.Body).Decode(&breaches); err != nil {
		return nil, err
	}

	return breaches, nil
}

func (p *DataBreachesPlugin) String() string {
	return "DataBreachesPlugin"
}
