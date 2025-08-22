package display

import (
	"fmt"
	"os"
	"sort"

	"github.com/olekukonko/tablewriter"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

// Display handles the presentation of trace results
type Display struct {
	output *os.File
}

// NewDisplay creates a new display instance
func NewDisplay(output *os.File) *Display {
	return &Display{
		output: output,
	}
}

// PrintTracesAsTable displays traces in a formatted table
func (d *Display) PrintTracesAsTable(traces []entities.Trace) {
	table := tablewriter.NewWriter(d.output)
	table.SetHeader([]string{"Value", "Type"})

	// Sort by type for consistent output
	sort.Slice(traces, func(i, j int) bool {
		return traces[i].Type < traces[j].Type
	})

	// Filter out empty values and add to table
	for _, trace := range traces {
		if trace.Value == "" {
			continue
		}
		table.Append([]string{trace.Value, string(trace.Type)})
	}

	table.Render()
}

// PrintSummary displays a summary of the processing results
func (d *Display) PrintSummary(totalTraces int, processedTraces int, errors int) {
	table := tablewriter.NewWriter(d.output)
	table.SetHeader([]string{"Metric", "Count"})

	table.Append([]string{"Total Traces", fmt.Sprintf("%d", totalTraces)})
	table.Append([]string{"Processed Traces", fmt.Sprintf("%d", processedTraces)})
	table.Append([]string{"Errors", fmt.Sprintf("%d", errors)})

	table.Render()
}
