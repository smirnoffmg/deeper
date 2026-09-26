// Command mcpd exposes deeper's scan service to MCP clients (AI agents).
//
// By default it serves the MCP Streamable HTTP transport at /mcp on a loopback
// address, so a client connects by URL. Pass -stdio to use the stdio transport
// instead (the client then launches mcpd as a subprocess and stdout is reserved
// for the JSON-RPC protocol). Diagnostics always go to stderr.
package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

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

func newServer(svc *scanservice.Service) *mcp.Server {
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

	return server
}

func main() {
	// Loopback by default: an MCP server that can trigger scans (active_scan
	// contacts targets directly) must not be reachable from the network unless
	// the operator deliberately puts auth in front of it.
	addr := flag.String("addr", "127.0.0.1:8765", "listen address for the Streamable HTTP endpoint (/mcp)")
	useStdio := flag.Bool("stdio", false, "serve over stdio instead of HTTP (client launches mcpd as a subprocess)")
	flag.Parse()

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
	server := newServer(svc)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if *useStdio {
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
			log.Fatal().Err(err).Msg("mcp server terminated")
		}
		return
	}

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Info().Str("addr", *addr).Msg("deeper MCP server listening (Streamable HTTP at /mcp)")
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal().Err(err).Msg("http server terminated")
	}
}
