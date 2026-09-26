// Command mcpd exposes deeper's scan service to MCP clients (AI agents) over
// stdio. stdout is reserved for the MCP JSON-RPC protocol; all diagnostics go
// to stderr.
package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/smirnoffmg/deeper/internal/app/deeper/scanservice"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
)

type scanInput struct {
	Seed string `json:"seed" jsonschema:"the seed identifier to scan: email, username, domain, IP, or company name"`
}

type listPluginsInput struct {
	Active bool `json:"active" jsonschema:"include active-recon plugins (port scan, directory brute force)"`
}

type listPluginsOutput struct {
	Plugins map[string][]string `json:"plugins"`
}

func main() {
	cfg := config.LoadConfig()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal().Err(err).Msg("resolve home directory")
	}
	dbPath := filepath.Join(homeDir, ".deeper", "deeper.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatal().Err(err).Msg("create database directory")
	}

	svc := scanservice.New(cfg, dbPath)

	server := mcp.NewServer(&mcp.Implementation{Name: "deeper", Version: "v1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "scan",
		Description: "Passive OSINT scan of a seed identifier (email, username, domain, IP, or company). Queries third-party sources only; does not contact the target directly.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in scanInput) (*mcp.CallToolResult, scanservice.Result, error) {
		result, err := svc.Scan(ctx, in.Seed, false)
		return nil, result, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "active_scan",
		Description: "Active-recon scan: in addition to passive sources it contacts the target directly (TCP port scan, directory brute force). Only use against targets you are authorized to actively probe.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in scanInput) (*mcp.CallToolResult, scanservice.Result, error) {
		result, err := svc.Scan(ctx, in.Seed, true)
		return nil, result, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_plugins",
		Description: "List registered plugins grouped by trace type.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in listPluginsInput) (*mcp.CallToolResult, listPluginsOutput, error) {
		m, err := svc.Plugins(in.Active)
		return nil, listPluginsOutput{Plugins: m}, err
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatal().Err(err).Msg("mcp server terminated")
	}
}
