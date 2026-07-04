package gravatar

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

const (
	publicProfileURL = "https://gravatar.com/%s.json"
	v3ProfileURL     = "https://api.gravatar.com/v3/profiles/%s"
)

type profileFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
	Do(req *http.Request) (*http.Response, error)
}

type gravatarProfile struct {
	displayName      string
	firstName        string
	lastName         string
	company          string
	verifiedAccounts []verifiedAccount
}

type verifiedAccount struct {
	accountType string
	url         string
}

func emailHash(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func fetchProfile(ctx context.Context, fetcher profileFetcher, hash, apiKey string) (*gravatarProfile, bool, error) {
	if apiKey != "" {
		profile, found, err := fetchV3Profile(ctx, fetcher, hash, apiKey)
		if err != nil || found {
			return profile, found, err
		}
	}

	return fetchPublicProfile(ctx, fetcher, hash)
}

func fetchV3Profile(ctx context.Context, fetcher profileFetcher, hash, apiKey string) (*gravatarProfile, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(v3ProfileURL, hash), nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := fetcher.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("gravatar v3 profile request failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}

	profile, ok := parseV3Profile(body)
	return profile, ok, nil
}

func fetchPublicProfile(ctx context.Context, fetcher profileFetcher, hash string) (*gravatarProfile, bool, error) {
	resp, err := fetcher.Get(ctx, fmt.Sprintf(publicProfileURL, hash))
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("gravatar profile request failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}

	profile, ok := parsePublicProfile(body)
	return profile, ok, nil
}

func parseV3Profile(body []byte) (*gravatarProfile, bool) {
	var raw struct {
		DisplayName      string `json:"display_name"`
		FirstName        string `json:"first_name"`
		LastName         string `json:"last_name"`
		Company          string `json:"company"`
		VerifiedAccounts []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"verified_accounts"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, false
	}

	profile := &gravatarProfile{
		displayName: strings.TrimSpace(raw.DisplayName),
		firstName:   strings.TrimSpace(raw.FirstName),
		lastName:    strings.TrimSpace(raw.LastName),
		company:     strings.TrimSpace(raw.Company),
	}
	for _, account := range raw.VerifiedAccounts {
		profile.verifiedAccounts = append(profile.verifiedAccounts, verifiedAccount{
			accountType: strings.ToLower(strings.TrimSpace(account.Type)),
			url:         strings.TrimSpace(account.URL),
		})
	}

	if profile.isEmpty() {
		return nil, false
	}
	return profile, true
}

func parsePublicProfile(body []byte) (*gravatarProfile, bool) {
	var raw struct {
		Entry []struct {
			DisplayName string `json:"displayName"`
			Accounts    []struct {
				Domain   string `json:"domain"`
				Username string `json:"username"`
				URL      string `json:"url"`
			} `json:"accounts"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, false
	}
	if len(raw.Entry) == 0 {
		return nil, false
	}

	entry := raw.Entry[0]
	profile := &gravatarProfile{
		displayName: strings.TrimSpace(entry.DisplayName),
	}
	for _, account := range entry.Accounts {
		accountURL := strings.TrimSpace(account.URL)
		if accountURL == "" && account.Domain != "" && account.Username != "" {
			accountURL = "https://" + account.Domain + "/" + account.Username
		}
		profile.verifiedAccounts = append(profile.verifiedAccounts, verifiedAccount{
			accountType: strings.ToLower(strings.TrimSpace(account.Domain)),
			url:         accountURL,
		})
	}

	if profile.isEmpty() {
		return nil, false
	}
	return profile, true
}

func (p *gravatarProfile) isEmpty() bool {
	return p.displayName == "" &&
		p.firstName == "" &&
		p.lastName == "" &&
		p.company == "" &&
		len(p.verifiedAccounts) == 0
}

func profileToTraces(profile *gravatarProfile, email string) []entities.Trace {
	seen := make(map[string]struct{})
	var traces []entities.Trace

	if name := profileName(profile); name != "" && !nameEchoesLocalPart(name, email) {
		key := string(entities.Name) + "\x00" + name
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			traces = append(traces, entities.Trace{Type: entities.Name, Value: name})
		}
	}

	if profile.company != "" {
		key := string(entities.Company) + "\x00" + profile.company
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			traces = append(traces, entities.Trace{Type: entities.Company, Value: profile.company})
		}
	}

	for _, account := range profile.verifiedAccounts {
		trace, ok := verifiedAccountTrace(account)
		if !ok {
			continue
		}
		key := string(trace.Type) + "\x00" + trace.Value
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		traces = append(traces, trace)
	}

	return traces
}

func profileName(profile *gravatarProfile) string {
	firstLast := strings.TrimSpace(strings.TrimSpace(profile.firstName + " " + profile.lastName))
	if firstLast != "" {
		return firstLast
	}
	return profile.displayName
}

func nameEchoesLocalPart(name, email string) bool {
	local, _, ok := strings.Cut(email, "@")
	if !ok {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(local))
}

type socialMatcher struct {
	hostSuffix string
	traceType  entities.TraceType
}

var socialMatchers = []socialMatcher{
	{hostSuffix: "twitter.com", traceType: entities.Twitter},
	{hostSuffix: "x.com", traceType: entities.Twitter},
	{hostSuffix: "github.com", traceType: entities.Github},
	{hostSuffix: "linkedin.com", traceType: entities.Linkedin},
	{hostSuffix: "instagram.com", traceType: entities.Instagram},
	{hostSuffix: "facebook.com", traceType: entities.Facebook},
	{hostSuffix: "tiktok.com", traceType: entities.TikTok},
	{hostSuffix: "reddit.com", traceType: entities.Reddit},
	{hostSuffix: "youtube.com", traceType: entities.YouTube},
	{hostSuffix: "pinterest.com", traceType: entities.Pinterest},
	{hostSuffix: "snapchat.com", traceType: entities.Snapchat},
	{hostSuffix: "tumblr.com", traceType: entities.Tumblr},
	{hostSuffix: "t.me", traceType: entities.SocialGeneric},
}

var accountTypeMatchers = map[string]entities.TraceType{
	"twitter":   entities.Twitter,
	"x":         entities.Twitter,
	"github":    entities.Github,
	"linkedin":  entities.Linkedin,
	"instagram": entities.Instagram,
	"facebook":  entities.Facebook,
	"tiktok":    entities.TikTok,
	"reddit":    entities.Reddit,
	"youtube":   entities.YouTube,
	"pinterest": entities.Pinterest,
	"snapchat":  entities.Snapchat,
	"tumblr":    entities.Tumblr,
	"telegram":  entities.SocialGeneric,
}

func verifiedAccountTrace(account verifiedAccount) (entities.Trace, bool) {
	if account.url != "" {
		if trace, ok := socialURLTrace(account.url); ok {
			return trace, true
		}
	}

	accountType := strings.TrimSuffix(account.accountType, ".com")
	if traceType, ok := accountTypeMatchers[accountType]; ok && account.url != "" {
		return entities.Trace{Value: normalizeSocialURL(account.url), Type: traceType}, true
	}

	return entities.Trace{}, false
}

func socialURLTrace(href string) (entities.Trace, bool) {
	parsed, err := url.Parse(href)
	if err != nil || parsed.Host == "" {
		return entities.Trace{}, false
	}

	host := strings.ToLower(parsed.Hostname())
	for _, matcher := range socialMatchers {
		if host == matcher.hostSuffix || strings.HasSuffix(host, "."+matcher.hostSuffix) {
			return entities.Trace{
				Value: normalizeSocialURL(href),
				Type:  matcher.traceType,
			}, true
		}
	}

	return entities.Trace{}, false
}

func normalizeSocialURL(href string) string {
	parsed, err := url.Parse(href)
	if err != nil {
		return strings.TrimSuffix(href, "/")
	}
	parsed.Fragment = ""
	parsed.RawQuery = ""
	return strings.TrimSuffix(parsed.String(), "/")
}
