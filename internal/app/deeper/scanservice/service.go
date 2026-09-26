// Package scanservice runs a deeper scan and returns structured results. It is
// a composition point that wires config, the plugin catalog, the engine and the
// database together, with no dependency on any MCP SDK so it can be driven by
// cmd/mcpd or any other caller.
package scanservice

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/smirnoffmg/deeper/internal/app/deeper/engine"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/metrics"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/catalog"
)

type Trace struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

type Result struct {
	Seed   string  `json:"seed"`
	Count  int     `json:"count"`
	Traces []Trace `json:"traces"`
}

type Service struct {
	cfg    *config.Config
	dbPath string
}

func New(cfg *config.Config, dbPath string) *Service {
	return &Service{cfg: cfg, dbPath: dbPath}
}

// Scan runs one scan of seed. When active is true, active-recon plugins are
// enabled (they contact the target directly); otherwise the scan is passive.
func (s *Service) Scan(ctx context.Context, seed string, active bool) (Result, error) {
	seed = strings.TrimSpace(seed)
	if seed == "" {
		return Result{}, fmt.Errorf("scan seed must not be empty")
	}

	// Copy the config per call so concurrent callers don't race on ActiveScan.
	cfg := *s.cfg
	cfg.ActiveScan = active

	registry, err := catalog.New(&cfg)
	if err != nil {
		log.Warn().Err(err).Msg("some plugins failed to register")
	}

	db, err := database.NewDatabase(s.dbPath)
	if err != nil {
		return Result{}, fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	repo := database.NewRepository(db)
	cache := database.NewCache(repo)

	session, err := repo.CreateScanSession(seed)
	if err != nil {
		return Result{}, fmt.Errorf("create scan session: %w", err)
	}

	eng := engine.NewEngine(&cfg, registry, metrics.GetGlobalMetrics(), repo, cache)
	defer func() { _ = eng.Shutdown(5 * time.Second) }()

	traces, err := eng.ProcessInput(ctx, seed, session.ID)
	if err != nil {
		return Result{}, fmt.Errorf("process input: %w", err)
	}

	out := make([]Trace, 0, len(traces))
	for _, t := range traces {
		out = append(out, Trace{Value: t.Value, Type: string(t.Type)})
	}

	return Result{Seed: seed, Count: len(out), Traces: out}, nil
}

// Plugins returns the registered plugin names grouped by trace type, honouring
// the active flag (active-recon plugins are only included when active is true).
func (s *Service) Plugins(active bool) (map[string][]string, error) {
	cfg := *s.cfg
	cfg.ActiveScan = active

	registry, err := catalog.New(&cfg)
	if err != nil {
		log.Warn().Err(err).Msg("some plugins failed to register")
	}

	result := make(map[string][]string, len(registry))
	for traceType, ps := range registry {
		names := make([]string, 0, len(ps))
		for _, p := range ps {
			names = append(names, p.String())
		}
		result[string(traceType)] = names
	}

	return result, nil
}
