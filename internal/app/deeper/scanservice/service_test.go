package scanservice

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
)

func newService(t *testing.T) *Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	return New(config.DefaultConfig(), dbPath)
}

func flatten(m map[string][]string) []string {
	var names []string
	for _, ps := range m {
		names = append(names, ps...)
	}
	return names
}

func TestScan_EmptySeed(t *testing.T) {
	s := newService(t)
	_, err := s.Scan(context.Background(), "  ", false)
	require.Error(t, err)
}

func TestScan_SeedWithNoPlugins_ReturnsSeedOnly(t *testing.T) {
	s := newService(t)

	// A phone-shaped seed guesses to entities.Phone, for which no plugin is
	// registered, so ProcessInput touches no network and yields just the seed.
	const seed = "+15550001234"
	require.NotContains(t, s.mustPlugins(t, false), "phone")

	result, err := s.Scan(context.Background(), seed, false)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Count, 1)

	values := make([]string, 0, len(result.Traces))
	for _, tr := range result.Traces {
		values = append(values, tr.Value)
	}
	assert.Contains(t, values, seed)
}

func TestPlugins_PassiveExcludesActive(t *testing.T) {
	s := newService(t)
	names := flatten(s.mustPlugins(t, false))
	assert.NotContains(t, names, "PortScanPlugin")
	assert.NotContains(t, names, "ContentDiscoveryPlugin")
	assert.Contains(t, names, "WhoisPlugin")
}

func TestPlugins_ActiveIncludesActive(t *testing.T) {
	s := newService(t)
	names := flatten(s.mustPlugins(t, true))
	assert.Contains(t, names, "PortScanPlugin")
	assert.Contains(t, names, "ContentDiscoveryPlugin")
}

func (s *Service) mustPlugins(t *testing.T, active bool) map[string][]string {
	t.Helper()
	m, err := s.Plugins(active)
	require.NoError(t, err)
	return m
}
