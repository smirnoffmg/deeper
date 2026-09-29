package social_profiles

import (
	"context"
	"fmt"
	"sync"
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

func TestParseSherlockData_RegexCheckFiltersUsernames(t *testing.T) {
	data := []byte(`{
		"SiteC": {
			"errorType": "status_code",
			"regexCheck": "^[a-z0-9_]{3,20}$",
			"url": "https://c.example/{}",
			"urlMain": "https://c.example/"
		}
	}`)

	entries, err := parseSherlockData(data)

	require.NoError(t, err)
	require.Contains(t, entries, "SiteC")
	assert.True(t, entries["SiteC"].allows("alsmirn"))
	assert.False(t, entries["SiteC"].allows("registry-one.example.com"))
}

// Some sherlock patterns use lookarounds, which RE2 cannot compile; such
// sites fall back to being probed rather than silently dropped.
func TestParseSherlockData_UnsupportedRegexCheckAllowsAll(t *testing.T) {
	data := []byte(`{
		"SiteD": {
			"errorType": "status_code",
			"regexCheck": "^(?!-)[a-z-]+$",
			"url": "https://d.example/{}",
			"urlMain": "https://d.example/"
		}
	}`)

	entries, err := parseSherlockData(data)

	require.NoError(t, err)
	require.Contains(t, entries, "SiteD")
	assert.True(t, entries["SiteD"].allows("anything.at.all"))
}

func TestFollowTrace_SkipsSitesWhoseRegexCheckRejectsUsername(t *testing.T) {
	data := []byte(`{
		"Strict": {"errorType": "status_code", "regexCheck": "^[a-z]+$", "url": "https://strict.example/{}"},
		"Open":   {"errorType": "status_code", "url": "https://open.example/{}"}
	}`)
	entries, err := parseSherlockData(data)
	require.NoError(t, err)

	var probed []string
	var mu sync.Mutex
	checkFn := func(_ context.Context, entry SherlockEntry, username string) bool {
		mu.Lock()
		probed = append(probed, entry.Url)
		mu.Unlock()
		return true
	}
	p := &SocialProfilesPlugin{entries: entries, checkFn: checkFn}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: InputTraceType, Value: "john.doe"})

	require.NoError(t, err)
	assert.Equal(t, []string{"https://open.example/{}"}, probed)
	assert.Len(t, traces, 1)
}

func TestProbeUrl_PrefersUrlProbe(t *testing.T) {
	withProbe := SherlockEntry{Url: "https://site.example/{}", UrlProbe: "https://api.site.example/users/{}"}
	withoutProbe := SherlockEntry{Url: "https://site.example/{}"}

	assert.Equal(t, "https://api.site.example/users/alice", withProbe.probeUrl("alice"))
	assert.Equal(t, "https://site.example/alice", withoutProbe.probeUrl("alice"))
}
