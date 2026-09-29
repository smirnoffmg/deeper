package social_profiles

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	errorTypeStatusCode  = "status_code"
	errorTypeMessage     = "message"
	errorTypeResponseURL = "response_url"
)

// newProbeClient returns an HTTP client tuned for the given sherlock
// errorType. response_url sites signal "not found" by redirecting to an
// error page; sherlock disables redirects for this detection method and
// checks the un-followed status instead of the (possibly 200) page it
// redirects to. Every other errorType follows redirects normally.
func newProbeClient(errorType string) *http.Client {
	client := &http.Client{Timeout: 5 * time.Second}
	if errorType == errorTypeResponseURL {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client
}

// decideByStatusCode implements sherlock's status_code detection: claimed
// iff the status is 2xx and (no errorCode is defined, or the status doesn't
// match it).
func decideByStatusCode(errorCode *int, status int) bool {
	if status < 200 || status >= 300 {
		return false
	}
	return errorCode == nil || status != *errorCode
}

// decideByMessage implements sherlock's message detection: claimed iff none
// of errorMsg's strings appear in the body. Deliberately no status check,
// matching upstream.
func decideByMessage(errorMsgs []string, body []byte) bool {
	text := string(body)
	for _, msg := range errorMsgs {
		if strings.Contains(text, msg) {
			return false
		}
	}
	return true
}

// wafFingerprints are copied from sherlock's WAFHitMsgs: challenge pages
// that come back 200 and would otherwise read as a claimed profile.
var wafFingerprints = []string{
	`.loading-spinner{visibility:hidden}body.no-js .challenge-running{display:none}body.dark{background-color:#222;color:#d9d9d9}body.dark a{color:#fff}body.dark a:hover{color:#ee730a;text-decoration:underline}body.dark .lds-ring div{border-color:#999 transparent transparent}body.dark .font-red{color:#b20f03}body.dark`,
	`<span id="challenge-error-text">`,
	`AwsWafIntegration.forceRefreshToken`,
	`{return l.onPageView}}),Object.defineProperty(r,"perimeterxIdentifiers",{enumerable:`,
}

func isWAFChallenge(body []byte) bool {
	for _, fp := range wafFingerprints {
		if bytes.Contains(body, []byte(fp)) {
			return true
		}
	}
	return false
}

// decideExistence dispatches to the detection strategy named by the entry's
// errorType. An unrecognized/empty errorType is treated conservatively as
// not claimed -- there are currently no such entries in sherlock's data.
func decideExistence(entry SherlockEntry, status int, body []byte) bool {
	if isWAFChallenge(body) {
		return false
	}
	switch entry.ErrorType {
	case errorTypeStatusCode:
		return decideByStatusCode(entry.ErrorCode, status)
	case errorTypeResponseURL:
		return status >= 200 && status < 300
	case errorTypeMessage:
		return decideByMessage(entry.ErrorMsg, body)
	default:
		return false
	}
}

func (e SherlockEntry) CheckUrl(ctx context.Context, username string) bool {
	url := e.probeUrl(username)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	req.Header.Set("Referer", e.UrlMain)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/116.0")

	client := newProbeClient(e.ErrorType)
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	// Non-message types only need the body for WAF fingerprints, which sit
	// near the top of a small challenge page.
	reader := io.Reader(resp.Body)
	if e.ErrorType != errorTypeMessage {
		reader = io.LimitReader(resp.Body, 64<<10)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return false
	}

	return decideExistence(e, resp.StatusCode, body)
}
