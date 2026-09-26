package social_profiles

import (
	"context"
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
	checkFn := func(_ context.Context, entry SherlockEntry, username string) bool { return true }
	p := &SocialProfilesPlugin{entries: manyEntries(50), checkFn: checkFn}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: InputTraceType, Value: "alsmirn"})

	require.NoError(t, err)
	assert.Len(t, traces, 50)
}

func TestFollowTrace_BoundsConcurrency(t *testing.T) {
	var current, maxSeen int32
	checkFn := func(_ context.Context, entry SherlockEntry, username string) bool {
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

	_, err := p.FollowTrace(context.Background(), entities.Trace{Type: InputTraceType, Value: "alsmirn"})

	require.NoError(t, err)
	assert.LessOrEqual(t, int(maxSeen), maxConcurrentChecks)
	assert.Greater(t, int(maxSeen), 1)
}

func TestFollowTrace_WrongTraceType(t *testing.T) {
	p := &SocialProfilesPlugin{entries: manyEntries(3)}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})

	require.NoError(t, err)
	assert.Nil(t, traces)
}

// TestFollowTrace_StopsAtDeadline is a regression test: ~480 sherlock probes
// used to run to completion no matter how long past TaskTimeout they went.
func TestFollowTrace_StopsAtDeadline(t *testing.T) {
	var calls atomic.Int64
	checkFn := func(ctx context.Context, entry SherlockEntry, username string) bool {
		calls.Add(1)
		<-ctx.Done()
		return false
	}
	p := &SocialProfilesPlugin{entries: manyEntries(200), checkFn: checkFn}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := p.FollowTrace(ctx, entities.Trace{Type: InputTraceType, Value: "alsmirn"})
	require.NoError(t, err)
	require.Less(t, time.Since(start), time.Second)
	require.LessOrEqual(t, calls.Load(), int64(maxConcurrentChecks+1), "no new probes may start after the deadline")
}
