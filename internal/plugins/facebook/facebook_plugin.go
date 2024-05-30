package facebook

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/state"
)

const InputTraceType = entities.Username

func init() {
	p := NewPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type FacebookPlugin struct{}

func NewPlugin() *FacebookPlugin {
	return &FacebookPlugin{}
}

func (g *FacebookPlugin) Register() error {
	state.RegisterPlugin(InputTraceType, g)
	return nil
}

func (g *FacebookPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	query := strings.ReplaceAll(trace.Value, " ", "+") + "+site:facebook.com"
	url := fmt.Sprintf("https://www.google.com/search?q=%s", query)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	profiles := parseGoogleResults(string(body))

	var newTraces []entities.Trace
	for _, profile := range profiles {
		newTraces = append(newTraces, entities.Trace{
			Value: profile,
			Type:  entities.Url,
		})
	}

	return newTraces, nil
}

func parseGoogleResults(body string) []string {
	var profiles []string
	// Simple string matching to extract profile URLs (more robust parsing may be needed)
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if strings.Contains(line, "https://www.facebook.com/") {
			start := strings.Index(line, "https://www.facebook.com/")
			end := strings.Index(line[start:], "\"")
			profileUrl := line[start : start+end]
			profiles = append(profiles, profileUrl)
		}
	}
	return profiles
}

func (g FacebookPlugin) String() string {
	return "FacebookPlugin"
}
