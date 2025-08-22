package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/smirnoffmg/deeper/internal/pkg/metrics"
)

var (
	metricsCmd = &cobra.Command{
		Use:   "metrics",
		Short: "Display application metrics and statistics",
		Long: `Display comprehensive metrics and statistics about the application's performance,
including trace processing, plugin execution, and system health.

Examples:
  deeper metrics
  deeper metrics --format json
  deeper metrics --format table
  deeper metrics --live`,
		RunE: runMetrics,
	}

	metricsFormat string
	metricsLive   bool
)

func init() {
	metricsCmd.Flags().StringVar(&metricsFormat, "format", "table", "output format (table, json)")
	metricsCmd.Flags().BoolVar(&metricsLive, "live", false, "display live metrics updates")
}

func runMetrics(cmd *cobra.Command, args []string) error {
	collector := metrics.GetGlobalMetrics()
	summary := collector.GetSummary()

	switch metricsFormat {
	case "json":
		return outputMetricsJSON(summary)
	case "table":
		return outputMetricsTable(summary)
	default:
		return fmt.Errorf("unsupported format: %s", metricsFormat)
	}
}

func outputMetricsJSON(summary *metrics.Summary) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summary)
}

func outputMetricsTable(summary *metrics.Summary) error {
	// Overall metrics
	fmt.Println("=== OVERALL METRICS ===")
	overallTable := tablewriter.NewWriter(os.Stdout)
	overallTable.SetHeader([]string{"Metric", "Value"})
	overallTable.SetBorder(false)
	overallTable.SetColumnSeparator("")
	overallTable.SetRowSeparator("")
	overallTable.SetCenterSeparator("")

	overallTable.Append([]string{"Uptime", formatDuration(summary.Uptime)})
	overallTable.Append([]string{"Traces Processed", fmt.Sprintf("%d", summary.TracesProcessed)})
	overallTable.Append([]string{"Traces Discovered", fmt.Sprintf("%d", summary.TracesDiscovered)})
	overallTable.Append([]string{"Plugin Executions", fmt.Sprintf("%d", summary.PluginExecutions)})
	overallTable.Append([]string{"Plugin Errors", fmt.Sprintf("%d", summary.PluginErrors)})
	overallTable.Append([]string{"Network Requests", fmt.Sprintf("%d", summary.NetworkRequests)})
	overallTable.Append([]string{"Network Errors", fmt.Sprintf("%d", summary.NetworkErrors)})
	overallTable.Append([]string{"Success Rate", fmt.Sprintf("%.2f%%", summary.SuccessRate)})
	overallTable.Append([]string{"Avg Processing Time", formatDuration(summary.AvgProcessingTime)})
	overallTable.Append([]string{"Requests/Second", fmt.Sprintf("%.2f", summary.RequestsPerSecond)})
	overallTable.Append([]string{"Error Rate", fmt.Sprintf("%.2f%%", summary.ErrorRate)})

	overallTable.Render()
	fmt.Println()

	// Trace type metrics
	if len(summary.TraceTypes) > 0 {
		fmt.Println("=== TRACE TYPE METRICS ===")
		traceTable := tablewriter.NewWriter(os.Stdout)
		traceTable.SetHeader([]string{"Trace Type", "Processed", "Discovered", "Success Rate", "Avg Time"})

		for traceType, metrics := range summary.TraceTypes {
			traceTable.Append([]string{
				string(traceType),
				fmt.Sprintf("%d", metrics.Processed),
				fmt.Sprintf("%d", metrics.Discovered),
				fmt.Sprintf("%.2f%%", metrics.SuccessRate),
				formatDuration(metrics.AvgTime),
			})
		}

		traceTable.Render()
		fmt.Println()
	}

	// Plugin metrics
	if len(summary.Plugins) > 0 {
		fmt.Println("=== PLUGIN METRICS ===")
		pluginTable := tablewriter.NewWriter(os.Stdout)
		pluginTable.SetHeader([]string{"Plugin", "Executions", "Errors", "Success Rate", "Avg Time", "Last Execution"})

		for pluginName, pluginMetrics := range summary.Plugins {
			pluginTable.Append([]string{
				pluginName,
				fmt.Sprintf("%d", pluginMetrics.Executions),
				fmt.Sprintf("%d", pluginMetrics.Errors),
				fmt.Sprintf("%.2f%%", pluginMetrics.SuccessRate),
				formatDuration(pluginMetrics.AvgTime),
				pluginMetrics.LastExecution.Format("15:04:05"),
			})
		}

		pluginTable.Render()
	}

	return nil
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	} else if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
	} else {
		return d.String()
	}
}
