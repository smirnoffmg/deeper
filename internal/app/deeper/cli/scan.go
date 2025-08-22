package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

var (
	scanDepth   int
	scanFilters []string
	scanSave    string
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

		// Create engine and display
		engine := createEngine()
		display := createDisplay()

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Process the input
		startTime := time.Now()
		traces, err := engine.ProcessInput(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to process input: %w", err)
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

		return nil
	},
}

func init() {
	scanCmd.Flags().IntVar(&scanDepth, "depth", 0, "maximum scan depth (0 for unlimited)")
	scanCmd.Flags().StringSliceVar(&scanFilters, "filter", []string{}, "filter results by trace types (comma-separated)")
	scanCmd.Flags().StringVar(&scanSave, "save", "", "save results to file")
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
