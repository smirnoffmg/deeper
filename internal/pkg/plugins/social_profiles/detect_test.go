package social_profiles

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecideByStatusCode(t *testing.T) {
	tests := []struct {
		name      string
		errorCode *int
		status    int
		want      bool
	}{
		{"2xx no errorCode is claimed", nil, 200, true},
		{"201 no errorCode is claimed", nil, 201, true},
		{"404 no errorCode is not claimed", nil, 404, false},
		{"3xx no errorCode is not claimed", nil, 301, false},
		{"2xx matching errorCode is not claimed", new(204), 204, false},
		{"2xx not matching errorCode is claimed", new(404), 200, true},
		{"non-2xx matching errorCode is not claimed", new(404), 404, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, decideByStatusCode(tt.errorCode, tt.status))
		})
	}
}

func TestDecideByMessage(t *testing.T) {
	tests := []struct {
		name      string
		errorMsgs []string
		body      string
		want      bool
	}{
		{"no error message present is claimed", []string{"not found"}, "welcome to the profile", true},
		{"error message present is not claimed", []string{"not found"}, "sorry, not found here", false},
		{"second of multiple messages matches", []string{"nope", "no such user"}, "error: no such user", false},
		{"empty errorMsg list is claimed", nil, "anything", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, decideByMessage(tt.errorMsgs, []byte(tt.body)))
		})
	}
}

func TestDecideExistence_DispatchesByErrorType(t *testing.T) {
	tests := []struct {
		name   string
		entry  SherlockEntry
		status int
		body   string
		want   bool
	}{
		{
			name:   "status_code type uses status only",
			entry:  SherlockEntry{ErrorType: "status_code"},
			status: 200,
			body:   "irrelevant",
			want:   true,
		},
		{
			name:   "status_code type with errorCode match is not claimed",
			entry:  SherlockEntry{ErrorType: "status_code", ErrorCode: new(404)},
			status: 404,
			want:   false,
		},
		{
			name:   "message type uses body only, ignores status",
			entry:  SherlockEntry{ErrorType: "message", ErrorMsg: []string{"not found"}},
			status: 403,
			body:   "welcome!",
			want:   true,
		},
		{
			name:   "message type with error text present",
			entry:  SherlockEntry{ErrorType: "message", ErrorMsg: []string{"not found"}},
			status: 200,
			body:   "not found",
			want:   false,
		},
		{
			// Regression case for the real bug: response_url sites redirect to
			// an error page on "not found". Sherlock disables redirects and
			// checks the un-followed status, so a 3xx here means absent.
			name:   "response_url type: 3xx (redirect not followed) is not claimed",
			entry:  SherlockEntry{ErrorType: "response_url"},
			status: 302,
			want:   false,
		},
		{
			name:   "response_url type: 2xx is claimed",
			entry:  SherlockEntry{ErrorType: "response_url"},
			status: 200,
			want:   true,
		},
		{
			name:   "unrecognized errorType defaults to not claimed",
			entry:  SherlockEntry{ErrorType: "something_new"},
			status: 200,
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, decideExistence(tt.entry, tt.status, []byte(tt.body)))
		})
	}
}

func TestNewProbeClient_ResponseURLDisablesRedirects(t *testing.T) {
	client := newProbeClient("response_url")

	assert.NotNil(t, client.CheckRedirect)
	err := client.CheckRedirect(&http.Request{}, nil)
	assert.Equal(t, http.ErrUseLastResponse, err)
}

func TestNewProbeClient_OtherTypesFollowRedirectsNormally(t *testing.T) {
	for _, errorType := range []string{"status_code", "message", ""} {
		t.Run(errorType, func(t *testing.T) {
			client := newProbeClient(errorType)
			assert.Nil(t, client.CheckRedirect)
		})
	}
}
