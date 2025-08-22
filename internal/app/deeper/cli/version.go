package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// These will be set by ldflags during build
	Version    = "dev"
	CommitHash = "unknown"
	BuildTime  = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version, build information, and system details for Deeper.`,
	Run: func(cmd *cobra.Command, args []string) {
		showVersion()
	},
}

func showVersion() {
	fmt.Printf("Deeper OSINT Tool\n")
	fmt.Printf("=================\n\n")
	fmt.Printf("Version:      %s\n", Version)
	fmt.Printf("Commit:       %s\n", CommitHash)
	fmt.Printf("Build Time:   %s\n", BuildTime)
	fmt.Printf("Go Version:   %s\n", runtime.Version())
	fmt.Printf("OS/Arch:      %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Compiler:     %s\n", runtime.Compiler)
}
