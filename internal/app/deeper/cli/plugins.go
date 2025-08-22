package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
)

// pluginsCmd represents the plugins command
var pluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "Manage and inspect plugins",
	Long: `The plugins command allows you to list, inspect, and manage the available 
plugins in the Deeper system. Each plugin specializes in processing specific 
trace types and discovering new traces.`,
}

// pluginsListCmd lists all available plugins
var pluginsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available plugins",
	Long: `List all plugins currently registered in the system, showing which 
trace types they handle and their current status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return listPlugins()
	},
}

// pluginsInfoCmd shows detailed information about a specific plugin
var pluginsInfoCmd = &cobra.Command{
	Use:   "info [plugin-name]",
	Short: "Show detailed information about a specific plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pluginName := args[0]
		return showPluginInfo(pluginName)
	},
}

// pluginsTypesCmd lists all supported trace types
var pluginsTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List all supported trace types",
	Long:  `List all trace types that can be processed by the available plugins.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return listTraceTypes()
	},
}

func init() {
	pluginsCmd.AddCommand(pluginsListCmd)
	pluginsCmd.AddCommand(pluginsInfoCmd)
	pluginsCmd.AddCommand(pluginsTypesCmd)
}

func listPlugins() error {
	fmt.Println("Available Plugins:")
	fmt.Println("==================")

	if len(state.ActivePlugins) == 0 {
		fmt.Println("No plugins registered")
		return nil
	}

	// Create table
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Trace Type", "Plugin Count", "Plugins"})
	table.SetBorder(true)

	// Sort trace types for consistent output
	var traceTypes []string
	for traceType := range state.ActivePlugins {
		traceTypes = append(traceTypes, string(traceType))
	}
	sort.Strings(traceTypes)

	totalPlugins := 0
	for _, traceTypeStr := range traceTypes {
		traceType := entities.TraceType(traceTypeStr)
		plugins := state.ActivePlugins[traceType]

		var pluginNames []string
		for _, plugin := range plugins {
			pluginNames = append(pluginNames, plugin.String())
		}

		table.Append([]string{
			string(traceType),
			fmt.Sprintf("%d", len(plugins)),
			strings.Join(pluginNames, ", "),
		})

		totalPlugins += len(plugins)
	}

	table.Render()

	fmt.Printf("\nSummary: %d trace types supported by %d plugins\n", len(traceTypes), totalPlugins)
	return nil
}

func showPluginInfo(pluginName string) error {
	fmt.Printf("Plugin Information: %s\n", pluginName)
	fmt.Println("====================")

	found := false
	for traceType, plugins := range state.ActivePlugins {
		for _, plugin := range plugins {
			if plugin.String() == pluginName {
				found = true
				fmt.Printf("Name: %s\n", plugin.String())
				fmt.Printf("Supported Trace Type: %s\n", traceType)
				fmt.Printf("Status: Active\n")

				// Try to get additional info (this would require extending the plugin interface)
				fmt.Printf("Description: Processes %s traces to discover related information\n", traceType)
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return fmt.Errorf("plugin '%s' not found", pluginName)
	}

	return nil
}

func listTraceTypes() error {
	fmt.Println("Supported Trace Types:")
	fmt.Println("======================")

	// Get all trace types from the entities package
	allTraceTypes := []entities.TraceType{
		entities.Email, entities.Phone, entities.Address, entities.IpAddr,
		entities.Domain, entities.Url, entities.Username, entities.Name,
		entities.Company, entities.Alias, entities.DateOfBirth, entities.Gender,
		entities.Nationality, entities.MacAddr, entities.SSHKey, entities.PGPKey,
		entities.BitcoinAddress, entities.PayPalAccount, entities.MedicalRecordNumber,
		entities.InsurancePolicy, entities.ExifData, entities.FileTimestamp,
		entities.Geolocation, entities.ForumRegistrations, entities.CommentsAndPosts,
		entities.NewsMentions, entities.CourtRecords, entities.Patents,
		entities.Publications, entities.EducationalInstitution, entities.Workplace,
		entities.Certificates, entities.ConferenceParticipation, entities.SocialGeneric,
		entities.Twitter, entities.Github, entities.Linkedin, entities.Instagram,
		entities.Facebook, entities.TikTok, entities.Reddit, entities.YouTube,
		entities.Pinterest, entities.Snapchat, entities.Tumblr, entities.Repository,
		entities.DnsRecordA, entities.DnsRecordAAAA, entities.DnsRecordMX,
		entities.DnsRecordNS, entities.DnsRecordTXT, entities.DnsRecordCNAME,
		entities.DnsRecordSOA, entities.DnsRecordPTR, entities.DnsRecordSRV,
		entities.DnsRecordCAA, entities.Whois, entities.Subdomain, entities.ASN,
		entities.Netblock, entities.Host, entities.IPRange,
	}

	// Create table showing which trace types have plugins
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Trace Type", "Status", "Plugin Count"})
	table.SetBorder(true)

	supported := 0
	for _, traceType := range allTraceTypes {
		pluginCount := len(state.ActivePlugins[traceType])
		status := "❌ Not Supported"
		if pluginCount > 0 {
			status = "✅ Supported"
			supported++
		}

		table.Append([]string{
			string(traceType),
			status,
			fmt.Sprintf("%d", pluginCount),
		})
	}

	table.Render()

	fmt.Printf("\nSummary: %d/%d trace types have plugin support\n", supported, len(allTraceTypes))
	return nil
}
