package social_profiles

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/state"
)

const InputTraceType = entities.Username

func init() {
	p := NewSocialProfilesPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type SherlockEntry struct {
	Url       string   `json:"url"`
	UrlMain   string   `json:"urlMain"`
	UrlProbe  string   `json:"urlProbe"`
	ErrorMsg  []string `json:"errorMsg,omitempty"`
	ErrorType string   `json:"errorType"`
}

func (e SherlockEntry) BuildUrl(username string) string {
	return strings.ReplaceAll(e.Url, "{}", username)
}

func (e *SherlockEntry) UnmarshalJSON(data []byte) error {
	// ErrorMsg could be a string or a list of strings

	type Alias SherlockEntry

	aux := &struct {
		ErrorMsg interface{} `json:"errorMsg,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	switch v := aux.ErrorMsg.(type) {
	case string:
		e.ErrorMsg = []string{v}
	case []interface{}:
		for _, i := range v {
			e.ErrorMsg = append(e.ErrorMsg, i.(string))
		}
	}

	return nil
}

func (e SherlockEntry) CheckUrl(username string) bool {
	url := e.BuildUrl(username)

	// we need to make a request in a context with a timeout

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}

	req.Header.Set("Referer", e.UrlMain)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/116.0")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	if resp.StatusCode != 200 {
		return false
	}

	// check body for error message

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return false
	}

	defer resp.Body.Close()

	for _, msg := range e.ErrorMsg {
		if strings.Contains(string(body), msg) {
			return false
		}
	}
	return true
}

type SocialProfilesPlugin struct {
	entries map[string]SherlockEntry
}

func NewSocialProfilesPlugin() *SocialProfilesPlugin {
	return &SocialProfilesPlugin{}
}

func (g *SocialProfilesPlugin) Register() error {
	// get latest data from sherlock
	jsonFileUrl := "https://raw.githubusercontent.com/sherlock-project/sherlock/master/sherlock/resources/data.json"

	resp, err := http.Get(jsonFileUrl)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	jsonFile, err := io.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	sherlockEntries := make(map[string]SherlockEntry)

	if err := json.Unmarshal(jsonFile, &sherlockEntries); err != nil {
		return err
	}

	log.Info().Msgf("Loaded %d entries from data.json", len(sherlockEntries))

	g.entries = sherlockEntries
	// Register the plugin

	state.RegisterPlugin(InputTraceType, g)
	return nil
}

func (g *SocialProfilesPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	var newTraces []entities.Trace

	var wg sync.WaitGroup

	for _, entry := range g.entries {
		wg.Add(1)

		go func(entry SherlockEntry) {
			defer wg.Done()

			if entry.CheckUrl(trace.Value) {
				newTraces = append(newTraces, entities.Trace{
					Value: entry.BuildUrl(trace.Value),
					Type:  entities.SocialGeneric,
				})
			}
		}(entry)

	}

	wg.Wait()
	return newTraces, nil
}

func (g SocialProfilesPlugin) String() string {
	return "SocialProfilesPlugin"
}
