package social_profiles

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
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

func manyEntries(n int) map[string]SherlockEntry {
	entries := make(map[string]SherlockEntry, n)
	for i := range n {
		entries[fmt.Sprintf("Site%d", i)] = SherlockEntry{Url: fmt.Sprintf("https://site%d.example/{}", i)}
	}
	return entries
}

func TestFollowTrace_CollectsAllMatchesWithoutDataRace(t *testing.T) {
	checkFn := func(entry SherlockEntry, username string) bool { return true }
	p := &SocialProfilesPlugin{entries: manyEntries(50), checkFn: checkFn}

	traces, err := p.FollowTrace(entities.Trace{Type: InputTraceType, Value: "alsmirn"})

	require.NoError(t, err)
	assert.Len(t, traces, 50)
}

func TestFollowTrace_BoundsConcurrency(t *testing.T) {
	var current, maxSeen int32
	checkFn := func(entry SherlockEntry, username string) bool {
		n := atomic.AddInt32(&current, 1)
		for {
			old := atomic.LoadInt32(&maxSeen)
			if n <= old || atomic.CompareAndSwapInt32(&maxSeen, old, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		atomic.AddInt32(&current, -1)
		return false
	}
	p := &SocialProfilesPlugin{entries: manyEntries(200), checkFn: checkFn}

	_, err := p.FollowTrace(entities.Trace{Type: InputTraceType, Value: "alsmirn"})

	require.NoError(t, err)
	assert.LessOrEqual(t, int(maxSeen), maxConcurrentChecks)
	assert.Greater(t, int(maxSeen), 1)
}

func TestFollowTrace_WrongTraceType(t *testing.T) {
	p := &SocialProfilesPlugin{entries: manyEntries(3)}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Domain, Value: "example.com"})

	require.NoError(t, err)
	assert.Nil(t, traces)
}
