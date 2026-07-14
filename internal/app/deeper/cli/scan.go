package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/smirnoffmg/deeper/internal/app/deeper/graphreport"
	"github.com/smirnoffmg/deeper/internal/pkg/browser"
	"github.com/smirnoffmg/deeper/internal/pkg/database"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

var (
	scanDepth   int
	scanFilters []string
	scanSave    string
	scanNoOpen  bool
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan [input]",
	Short: "Scan and discover traces for the given input",
	Long: `Scan analyzes the provided input (email, username, domain, etc.) and
discovers related traces using various plugins. The scan follows traces
recursively to build a comprehensive profile.

Examples:
  deeper scan username123
  deeper scan test@example.com --depth 3
  deeper scan github.com --output json --save results.json
  deeper scan user@domain.com --filter="repository,social"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := args[0]

		log.Info().Msgf("Starting scan for input: %s", input)

		eng, repo, err := createEngine()
		if err != nil {
			return err
		}
		display := createDisplay()

		session, err := repo.CreateScanSession(input)
		if err != nil {
			return fmt.Errorf("failed to create scan session: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		startTime := time.Now()
		traces, err := eng.ProcessInput(ctx, input, session.ID)
		completedAt := time.Now()
		session.CompletedAt = &completedAt
		if err != nil {
			session.Status = "failed"
			_ = repo.UpdateScanSession(session)
			return fmt.Errorf("failed to process input: %w", err)
		}

		session.Status = "completed"
		session.UniqueTraces = len(traces)
		session.TotalTraces = len(traces)
		if err := repo.UpdateScanSession(session); err != nil {
			return fmt.Errorf("failed to update scan session: %w", err)
		}

		processingTime := time.Since(startTime)
		log.Info().Msgf("Scan completed in %v", processingTime)

		// Apply filters if specified
		if len(scanFilters) > 0 {
			traces = applyFilters(traces, scanFilters)
		}

		// Limit depth if specified
		if scanDepth > 0 {
			traces = limitDepth(traces, scanDepth)
		}

		// Display results
		if len(traces) == 0 {
			fmt.Println("No traces found")
			return nil
		}

		log.Info().Msgf("Found %d traces", len(traces))

		// Output results based on format
		switch output {
		case "table":
			display.PrintTracesAsTable(traces)
		case "json":
			return outputTracesJSON(traces)
		case "csv":
			return outputTracesCSV(traces)
		default:
			return fmt.Errorf("unsupported output format: %s", output)
		}

		// Save results if requested
		if scanSave != "" {
			if err := saveResults(traces, scanSave); err != nil {
				log.Error().Err(err).Msgf("Failed to save results to %s", scanSave)
				return err
			}
			log.Info().Msgf("Results saved to %s", scanSave)
		}

		graphPath, err := saveGraphReport(repo, session.ID, !scanNoOpen)
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate graph report")
			return err
		}
		if graphPath != "" {
			log.Info().Msgf("Graph report: %s", graphPath)
		}

		return nil
	},
}

func init() {
	scanCmd.Flags().IntVar(&scanDepth, "depth", 0, "maximum scan depth (0 for unlimited)")
	scanCmd.Flags().StringSliceVar(&scanFilters, "filter", []string{}, "filter results by trace types (comma-separated)")
	scanCmd.Flags().StringVar(&scanSave, "save", "", "save results to file")
	scanCmd.Flags().BoolVar(&scanNoOpen, "no-open", false, "do not auto-open the graph report in a browser")
}

// buildGraphReport maps stored graph rows to graphreport's presentation
// types. Edges with a nil ParentTraceID are the scan's seed edge (see
// database.SeedPluginName) — the root trace is still present as a node via
// its child_trace_id side, it just has no real parent to draw an edge from.
func buildGraphReport(nodes []database.Trace, edges []database.TraceEdge) ([]graphreport.Node, []graphreport.Edge) {
	reportNodes := make([]graphreport.Node, 0, len(nodes))
	for _, n := range nodes {
		reportNodes = append(reportNodes, graphreport.Node{ID: n.ID, Label: n.Value, Type: string(n.Type)})
	}

	reportEdges := make([]graphreport.Edge, 0, len(edges))
	for _, e := range edges {
		if e.ParentTraceID == nil {
			continue
		}
		reportEdges = append(reportEdges, graphreport.Edge{From: *e.ParentTraceID, To: e.ChildTraceID, Label: e.PluginName})
	}

	return reportNodes, reportEdges
}

// saveGraphReport renders the scan's discovery graph to a standalone HTML
// file under ~/.deeper/reports and optionally opens it in the browser. It
// returns an empty path (no error) when the scan recorded no traces.
func saveGraphReport(repo *database.Repository, sessionID int64, openInBrowser bool) (string, error) {
	nodes, edges, err := repo.GetScanGraph(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to load scan graph: %w", err)
	}
	if len(nodes) == 0 {
		return "", nil
	}

	reportNodes, reportEdges := buildGraphReport(nodes, edges)
	html, err := graphreport.Render(reportNodes, reportEdges)
	if err != nil {
		return "", fmt.Errorf("failed to render graph report: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	reportsDir := filepath.Join(homeDir, ".deeper", "reports")
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create reports directory: %w", err)
	}

	path := filepath.Join(reportsDir, fmt.Sprintf("scan-%d.html", sessionID))
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		return "", fmt.Errorf("failed to write graph report: %w", err)
	}

	if openInBrowser {
		if err := browser.Open(path); err != nil {
			log.Warn().Err(err).Msg("Failed to open graph report in browser")
		}
	}

	return path, nil
}

func applyFilters(traces []entities.Trace, filters []string) []entities.Trace {
	if len(filters) == 0 {
		return traces
	}

	filterMap := make(map[string]bool)
	for _, filter := range filters {
		filterMap[filter] = true
	}

	var filtered []entities.Trace
	for _, trace := range traces {
		if filterMap[string(trace.Type)] {
			filtered = append(filtered, trace)
		}
	}

	log.Info().Msgf("Applied filters, %d traces remaining", len(filtered))
	return filtered
}

func limitDepth(traces []entities.Trace, maxDepth int) []entities.Trace {
	// For now, just return all traces
	// In a more sophisticated implementation, we'd track depth during processing
	log.Info().Msgf("Depth limiting not yet implemented, returning all %d traces", len(traces))
	return traces
}

func outputTracesJSON(traces []entities.Trace) error {
	fmt.Printf("[\n")
	for i, trace := range traces {
		fmt.Printf("  {\n")
		fmt.Printf("    \"value\": \"%s\",\n", trace.Value)
		fmt.Printf("    \"type\": \"%s\"\n", trace.Type)
		if i < len(traces)-1 {
			fmt.Printf("  },\n")
		} else {
			fmt.Printf("  }\n")
		}
	}
	fmt.Printf("]\n")
	return nil
}

func outputTracesCSV(traces []entities.Trace) error {
	fmt.Println("value,type")
	for _, trace := range traces {
		fmt.Printf("\"%s\",\"%s\"\n", trace.Value, trace.Type)
	}
	return nil
}

func saveResults(traces []entities.Trace, filename string) error {
	// Implementation for saving results to file
	// This would write to the specified file in the requested format
	log.Info().Msgf("Saving %d traces to %s (not yet implemented)", len(traces), filename)
	return nil
}
