package main

import (
	"os"
	"sort"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/olekukonko/tablewriter"
	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/plugins"

	_ "github.com/smirnoffmg/deeper/internal/plugins/academic_papers"
	_ "github.com/smirnoffmg/deeper/internal/plugins/coderepos"
	_ "github.com/smirnoffmg/deeper/internal/plugins/facebook"
	_ "github.com/smirnoffmg/deeper/internal/plugins/subdomains"
	"github.com/smirnoffmg/deeper/internal/state"
)

func checkTrace(trace entities.Trace) (result []entities.Trace) {
	var wg sync.WaitGroup
	for _, plugin := range state.ActivePlugins[trace.Type] {
		log.Info().Msgf("Checking trace %v with plugin %v", trace, plugin)
		wg.Add(1)
		go func(plugin plugins.DeeperPlugin) {
			defer wg.Done()
			newTraces, err := plugin.FollowTrace(trace)

			if err != nil {
				log.Error().Msgf("Error checking trace %v with plugin %v: %v", trace, plugin, err)
				return
			}

			// filter for empty values
			for _, newTrace := range newTraces {
				if newTrace.Value == "" {
					continue
				}
				result = append(result, newTrace)
			}

		}(plugin)
	}
	wg.Wait()
	return
}

func printTracesAsTable(traces []entities.Trace) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Value", "Type"})

	// sort by type
	sort.Slice(traces, func(i, j int) bool {
		return traces[i].Type < traces[j].Type
	})

	for _, trace := range traces {
		if trace.Value == "" {
			continue
		}
		table.Append([]string{trace.Value, string(trace.Type)})
	}

	table.Render()

}

func main() {
	firstTrace := os.Args[1]

	stack := []entities.Trace{entities.NewTrace(firstTrace)}

	seen := make(map[entities.Trace]bool)

	for len(stack) > 0 {
		trace := stack[0]
		stack = stack[1:]

		if _, ok := seen[trace]; ok {
			continue
		}

		seen[trace] = true

		stack = append(stack, checkTrace(trace)...)

	}

	var results []entities.Trace

	for trace := range seen {
		results = append(results, trace)
	}

	printTracesAsTable(results)

}
