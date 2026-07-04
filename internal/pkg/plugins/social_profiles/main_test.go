package social_profiles

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSherlockData_SkipsNonEntryMetadataKeys(t *testing.T) {
	data := []byte(`{
		"$schema": "data.schema.json",
		"GitHub": {
			"errorType": "status_code",
			"url": "https://github.com/{}",
			"urlMain": "https://github.com/",
			"username_claimed": "octocat"
		}
	}`)

	entries, err := parseSherlockData(data)

	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Contains(t, entries, "GitHub")
	assert.NotContains(t, entries, "$schema")
}

func TestParseSherlockData_ErrorMsgAsString(t *testing.T) {
	data := []byte(`{
		"SiteA": {
			"errorType": "message",
			"errorMsg": "not found",
			"url": "https://a.example/{}",
			"urlMain": "https://a.example/"
		}
	}`)

	entries, err := parseSherlockData(data)

	require.NoError(t, err)
	require.Contains(t, entries, "SiteA")
	assert.Equal(t, []string{"not found"}, entries["SiteA"].ErrorMsg)
}

func TestParseSherlockData_ErrorMsgAsList(t *testing.T) {
	data := []byte(`{
		"SiteB": {
			"errorType": "message",
			"errorMsg": ["not found", "no such user"],
			"url": "https://b.example/{}",
			"urlMain": "https://b.example/"
		}
	}`)

	entries, err := parseSherlockData(data)

	require.NoError(t, err)
	require.Contains(t, entries, "SiteB")
	assert.Equal(t, []string{"not found", "no such user"}, entries["SiteB"].ErrorMsg)
}
