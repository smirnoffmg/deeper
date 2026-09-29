package social_profiles

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"

	"golang.org/x/sync/errgroup"
)

const InputTraceType = entities.Username

type SherlockEntry struct {
	Url       string   `json:"url"`
	UrlMain   string   `json:"urlMain"`
	UrlProbe  string   `json:"urlProbe"`
	ErrorMsg  []string `json:"errorMsg,omitempty"`
	ErrorType string   `json:"errorType"`
	ErrorCode *int     `json:"errorCode,omitempty"`

	RegexCheck string `json:"regexCheck,omitempty"`
	// regexCheck is nil when the entry has no pattern or its pattern uses
	// syntax RE2 can't compile (sherlock's are Python regexes).
	regexCheck *regexp.Regexp
}

func (e SherlockEntry) BuildUrl(username string) string {
	return strings.ReplaceAll(e.Url, "{}", username)
}

func (e SherlockEntry) probeUrl(username string) string {
	if e.UrlProbe == "" {
		return e.BuildUrl(username)
	}
	return strings.ReplaceAll(e.UrlProbe, "{}", username)
}

// allows mirrors sherlock's ILLEGAL status: a username the site's own
// pattern rejects can't exist there, so probing it only invites false
// positives from sites that answer 200 for anything.
func (e SherlockEntry) allows(username string) bool {
	return e.regexCheck == nil || e.regexCheck.MatchString(username)
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

	if e.RegexCheck != "" {
		e.regexCheck, _ = regexp.Compile(e.RegexCheck)
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
	checkFn func(ctx context.Context, entry SherlockEntry, username string) bool
}

func NewSocialProfilesPlugin() *SocialProfilesPlugin {
	return &SocialProfilesPlugin{
		checkFn: func(ctx context.Context, entry SherlockEntry, username string) bool {
			return entry.CheckUrl(ctx, username)
		},
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

func (g *SocialProfilesPlugin) Register(r plugins.Registry) error {
	// get latest data from sherlock
	jsonFileUrl := "https://raw.githubusercontent.com/sherlock-project/sherlock/master/sherlock_project/resources/data.json"

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get(jsonFileUrl)

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

	r.Add(InputTraceType, g)
	return nil
}

func (g *SocialProfilesPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	var (
		mu        sync.Mutex
		newTraces []entities.Trace
		eg        errgroup.Group
	)
	eg.SetLimit(maxConcurrentChecks)

	for _, entry := range g.entries {
		// A non-matching site is a normal result, not a group error, so the
		// closure returns nil; errgroup gives bounded concurrency + Wait.
		if ctx.Err() != nil {
			break
		}
		if !entry.allows(trace.Value) {
			continue
		}
		eg.Go(func() error {
			if g.checkFn(ctx, entry, trace.Value) {
				mu.Lock()
				newTraces = append(newTraces, entities.Trace{
					Value: entry.BuildUrl(trace.Value),
					Type:  entities.SocialGeneric,
				})
				mu.Unlock()
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return newTraces, nil
}

func (g SocialProfilesPlugin) String() string {
	return "SocialProfilesPlugin"
}
