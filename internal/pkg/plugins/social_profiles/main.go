package social_profiles

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
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
	ErrorCode *int     `json:"errorCode,omitempty"`
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

// maxConcurrentChecks bounds how many sherlock site checks run in parallel
// per FollowTrace call. Sherlock's data.json has ~480 entries; firing one
// unbounded goroutine per entry starved the shared worker pool (each of up
// to MaxConcurrency simultaneous Username traces fanned out independently)
// and could open ~2000 concurrent outbound connections for a handful of
// usernames.
const maxConcurrentChecks = 30

type SocialProfilesPlugin struct {
	entries map[string]SherlockEntry
	checkFn func(entry SherlockEntry, username string) bool
}

func NewSocialProfilesPlugin() *SocialProfilesPlugin {
	return &SocialProfilesPlugin{
		checkFn: func(entry SherlockEntry, username string) bool { return entry.CheckUrl(username) },
	}
}

// parseSherlockData decodes sherlock's data.json, skipping top-level keys
// that aren't site entries (e.g. the "$schema" metadata key added upstream).
func parseSherlockData(data []byte) (map[string]SherlockEntry, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	entries := make(map[string]SherlockEntry, len(raw))
	for name, entryData := range raw {
		var entry SherlockEntry
		if err := json.Unmarshal(entryData, &entry); err != nil {
			continue
		}
		entries[name] = entry
	}

	return entries, nil
}

func (g *SocialProfilesPlugin) Register() error {
	// get latest data from sherlock
	jsonFileUrl := "https://raw.githubusercontent.com/sherlock-project/sherlock/master/sherlock_project/resources/data.json"

	resp, err := http.Get(jsonFileUrl)

	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	jsonFile, err := io.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	sherlockEntries, err := parseSherlockData(jsonFile)
	if err != nil {
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

	var (
		mu        sync.Mutex
		newTraces []entities.Trace
		wg        sync.WaitGroup
		sem       = make(chan struct{}, maxConcurrentChecks)
	)

	for _, entry := range g.entries {
		wg.Add(1)
		sem <- struct{}{}

		go func(entry SherlockEntry) {
			defer wg.Done()
			defer func() { <-sem }()

			if g.checkFn(entry, trace.Value) {
				mu.Lock()
				newTraces = append(newTraces, entities.Trace{
					Value: entry.BuildUrl(trace.Value),
					Type:  entities.SocialGeneric,
				})
				mu.Unlock()
			}
		}(entry)

	}

	wg.Wait()
	return newTraces, nil
}

func (g SocialProfilesPlugin) String() string {
	return "SocialProfilesPlugin"
}
