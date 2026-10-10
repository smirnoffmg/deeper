package github_keys

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

type keyFetcher interface {
	Do(req *http.Request) (*http.Response, error)
}

func fetchSSHKeys(ctx context.Context, fetcher keyFetcher, username, token string) ([]entities.Trace, error) {
	reqURL := fmt.Sprintf("https://api.github.com/users/%s/keys", username)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := fetcher.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github ssh keys request failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var traces []entities.Trace
	for _, k := range raw {
		key := strings.TrimSpace(k.Key)
		if key == "" {
			continue
		}
		traces = append(traces, entities.Trace{Type: entities.SSHKey, Value: key})
	}
	return traces, nil
}

func fetchGPGKeys(ctx context.Context, fetcher keyFetcher, username, token string) ([]entities.Trace, error) {
	reqURL := fmt.Sprintf("https://api.github.com/users/%s/gpg_keys", username)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := fetcher.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github gpg keys request failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		KeyID  string `json:"key_id"`
		Emails []struct {
			Email    string `json:"email"`
			Verified bool   `json:"verified"`
		} `json:"emails"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var traces []entities.Trace
	for _, k := range raw {
		if keyID := strings.TrimSpace(k.KeyID); keyID != "" {
			traces = append(traces, entities.Trace{Type: entities.PGPKey, Value: keyID})
		}
		// Only verified emails: GitHub already checked ownership, so this
		// avoids trusting a spoofable, unverified claim (same noise-avoidance
		// discipline contact_crawler and companyregistry already apply).
		// Verification alone doesn't rule out a placeholder like
		// "you@example.com" though, so shape/reserved-domain validation
		// still runs on top via IsRealEmail.
		for _, e := range k.Emails {
			if e.Verified && entities.IsRealEmail(e.Email) {
				traces = append(traces, entities.Trace{Type: entities.Email, Value: e.Email})
			}
		}
	}
	return traces, nil
}

// githubLogin returns the login of a github.com account URL. Repo URLs share
// the Github trace type and are left to github_identity.
func githubLogin(value string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return "", false
	}
	login := strings.Trim(parsed.Path, "/")
	if login == "" || strings.Contains(login, "/") {
		return "", false
	}
	return login, true
}
