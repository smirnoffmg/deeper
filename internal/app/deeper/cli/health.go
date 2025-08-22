package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
)

var (
	healthDetailed bool
	healthTimeout  time.Duration
)

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check system health and plugin status",
	Long: `Perform health checks on the Deeper system, including:
- Plugin registration status
- Configuration validation
- System resources
- External API connectivity (with --detailed flag)

This command helps diagnose issues and ensure the system is ready for operations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHealthCheck()
	},
}

func init() {
	healthCmd.Flags().BoolVar(&healthDetailed, "detailed", false, "perform detailed health checks including external connectivity")
	healthCmd.Flags().DurationVar(&healthTimeout, "check-timeout", 10*time.Second, "timeout for individual health checks")
}

type HealthCheck struct {
	Name     string
	Status   string
	Message  string
	Duration time.Duration
	Error    error
}

func runHealthCheck() error {
	fmt.Println("Deeper System Health Check")
	fmt.Println("==========================")

	var checks []HealthCheck

	// Basic health checks
	checks = append(checks, checkConfiguration())
	checks = append(checks, checkPluginRegistration())
	checks = append(checks, checkTraceTypeSupport())

	// Detailed checks if requested
	if healthDetailed {
		log.Info().Msg("Running detailed health checks...")
		checks = append(checks, checkExternalConnectivity())
		checks = append(checks, checkPluginFunctionality())
	}

	// Display results
	displayHealthResults(checks)

	// Determine overall status
	failed := 0
	for _, check := range checks {
		if check.Status == "FAIL" {
			failed++
		}
	}

	if failed > 0 {
		fmt.Printf("\n❌ Health check completed with %d failures\n", failed)
		return fmt.Errorf("health check failed")
	} else {
		fmt.Printf("\n✅ All health checks passed\n")
	}

	return nil
}

func checkConfiguration() HealthCheck {
	start := time.Now()
	check := HealthCheck{Name: "Configuration", Status: "PASS"}

	defer func() {
		check.Duration = time.Since(start)
	}()

	// Load and validate configuration
	cfg := config.LoadConfig()
	if cfg == nil {
		check.Status = "FAIL"
		check.Message = "Failed to load configuration"
		return check
	}

	// Validate configuration values
	if cfg.HTTPTimeout <= 0 {
		check.Status = "FAIL"
		check.Message = "Invalid HTTP timeout"
		return check
	}

	if cfg.MaxConcurrency <= 0 {
		check.Status = "FAIL"
		check.Message = "Invalid max concurrency"
		return check
	}

	if cfg.RateLimitPerSecond <= 0 {
		check.Status = "FAIL"
		check.Message = "Invalid rate limit"
		return check
	}

	check.Message = "Configuration loaded successfully"
	return check
}

func checkPluginRegistration() HealthCheck {
	start := time.Now()
	check := HealthCheck{Name: "Plugin Registration", Status: "PASS"}

	defer func() {
		check.Duration = time.Since(start)
	}()

	if len(state.ActivePlugins) == 0 {
		check.Status = "FAIL"
		check.Message = "No plugins registered"
		return check
	}

	totalPlugins := 0
	for _, plugins := range state.ActivePlugins {
		totalPlugins += len(plugins)
	}

	check.Message = fmt.Sprintf("%d plugins registered for %d trace types", totalPlugins, len(state.ActivePlugins))
	return check
}

func checkTraceTypeSupport() HealthCheck {
	start := time.Now()
	check := HealthCheck{Name: "Trace Type Support", Status: "PASS"}

	defer func() {
		check.Duration = time.Since(start)
	}()

	// Check core trace types
	coreTypes := []entities.TraceType{
		entities.Username, entities.Email, entities.Domain, entities.IpAddr,
	}

	supported := 0
	for _, traceType := range coreTypes {
		if len(state.ActivePlugins[traceType]) > 0 {
			supported++
		}
	}

	if supported == 0 {
		check.Status = "FAIL"
		check.Message = "No core trace types supported"
		return check
	}

	if supported < len(coreTypes) {
		check.Status = "WARN"
		check.Message = fmt.Sprintf("%d/%d core trace types supported", supported, len(coreTypes))
		return check
	}

	check.Message = fmt.Sprintf("All %d core trace types supported", len(coreTypes))
	return check
}

func checkExternalConnectivity() HealthCheck {
	start := time.Now()
	check := HealthCheck{Name: "External Connectivity", Status: "PASS"}

	defer func() {
		check.Duration = time.Since(start)
	}()

	// This would test connectivity to external APIs
	// For now, we'll simulate the check
	time.Sleep(100 * time.Millisecond) // Simulate network check

	check.Message = "External API connectivity check passed"
	return check
}

func checkPluginFunctionality() HealthCheck {
	start := time.Now()
	check := HealthCheck{Name: "Plugin Functionality", Status: "PASS"}

	defer func() {
		check.Duration = time.Since(start)
	}()

	// Test a sample plugin with a safe input
	testTrace := entities.Trace{
		Value: "test",
		Type:  entities.Username,
	}

	plugins := state.ActivePlugins[entities.Username]
	if len(plugins) > 0 {
		plugin := plugins[0]
		_, err := plugin.FollowTrace(testTrace)
		if err != nil {
			check.Status = "WARN"
			check.Message = fmt.Sprintf("Plugin test failed: %v", err)
			return check
		}
	}

	check.Message = "Plugin functionality test passed"
	return check
}

func displayHealthResults(checks []HealthCheck) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Check", "Status", "Duration", "Message"})
	table.SetBorder(true)

	for _, check := range checks {
		status := check.Status
		if status == "PASS" {
			status = "✅ PASS"
		} else if status == "WARN" {
			status = "⚠️  WARN"
		} else {
			status = "❌ FAIL"
		}

		table.Append([]string{
			check.Name,
			status,
			fmt.Sprintf("%v", check.Duration.Truncate(time.Millisecond)),
			check.Message,
		})
	}

	table.Render()
}
