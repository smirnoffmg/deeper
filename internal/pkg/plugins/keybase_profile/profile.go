package keybase_profile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

type profileFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

// extractHandle returns the keybase username from a keybase.io profile URL,
// or "" if the URL isn't a single-segment keybase.io profile link.
func extractHandle(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() != "keybase.io" {
		return ""
	}
	handle := strings.Trim(parsed.Path, "/")
	if handle == "" || strings.Contains(handle, "/") {
		return ""
	}
	return handle
}

// recognizedProofTypes are keybase proof_type values that map cleanly onto
// another profile URL worth following further. Types like "dns" or "web"
// (arbitrary claimed websites) are excluded since they aren't reliably
// another social profile.
var recognizedProofTypes = map[string]bool{
	"github":     true,
	"reddit":     true,
	"twitter":    true,
	"hackernews": true,
	"gitlab":     true,
}

// bioAsURL returns bio normalized as a URL if it looks like one (e.g.
// "about.me/lig1"), or "" if it reads as free text.
func bioAsURL(bio string) string {
	bio = strings.TrimSpace(bio)
	if bio == "" {
		return ""
	}
	candidate := bio
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	parsed, err := url.Parse(candidate)
	if err != nil || !strings.Contains(parsed.Hostname(), ".") {
		return ""
	}
	return candidate
}

type keybaseResponse struct {
	Status struct {
		Code int `json:"code"`
	} `json:"status"`
	Them []*struct {
		Profile *struct {
			FullName string `json:"full_name"`
			Location string `json:"location"`
			Bio      string `json:"bio"`
		} `json:"profile"`
		ProofsSummary struct {
			All []struct {
				ProofType  string `json:"proof_type"`
				ServiceURL string `json:"service_url"`
			} `json:"all"`
		} `json:"proofs_summary"`
	} `json:"them"`
}

func fetchProfile(ctx context.Context, fetcher profileFetcher, handle string) ([]entities.Trace, error) {
	reqURL := "https://keybase.io/_/api/1.0/user/lookup.json?usernames=" + handle

	resp, err := fetcher.Get(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("keybase lookup failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw keybaseResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	// status.code != 0 (e.g. INPUT_ERROR for an unregistered/invalid
	// username) and a null entry in "them" (a syntactically valid but
	// nonexistent username) both mean "no such user" -- verified live
	// against both cases, not assumed.
	if raw.Status.Code != 0 || len(raw.Them) == 0 || raw.Them[0] == nil {
		return nil, nil
	}
	user := raw.Them[0]

	var traces []entities.Trace
	addTrace := func(traceType entities.TraceType, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		traces = append(traces, entities.Trace{Type: traceType, Value: value})
	}

	if user.Profile != nil {
		addTrace(entities.Name, user.Profile.FullName)
		addTrace(entities.Address, user.Profile.Location)
		addTrace(entities.Url, bioAsURL(user.Profile.Bio))
	}

	for _, proof := range user.ProofsSummary.All {
		if recognizedProofTypes[proof.ProofType] {
			addTrace(entities.SocialGeneric, proof.ServiceURL)
		}
	}

	return traces, nil
}
