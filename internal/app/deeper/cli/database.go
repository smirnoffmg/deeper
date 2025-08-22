package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/smirnoffmg/deeper/internal/pkg/database"
)

var (
	databaseCmd = &cobra.Command{
		Use:   "database",
		Short: "Manage database operations",
		Long: `Manage database operations including statistics, cleanup, and maintenance.

Examples:
  deeper database stats
  deeper database cleanup
  deeper database info`,
	}

	databaseStatsCmd = &cobra.Command{
		Use:   "stats",
		Short: "Show database statistics",
		Long:  "Display comprehensive statistics about the database including traces, sessions, and cache entries.",
		RunE:  runDatabaseStats,
	}

	databaseCleanupCmd = &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up expired cache entries",
		Long:  "Remove expired cache entries to free up database space.",
		RunE:  runDatabaseCleanup,
	}

	databaseInfoCmd = &cobra.Command{
		Use:   "info",
		Short: "Show database information",
		Long:  "Display database file information and location.",
		RunE:  runDatabaseInfo,
	}
)

func init() {
	databaseCmd.AddCommand(databaseStatsCmd)
	databaseCmd.AddCommand(databaseCleanupCmd)
	databaseCmd.AddCommand(databaseInfoCmd)
}

func runDatabaseStats(cmd *cobra.Command, args []string) error {
	// Create database connection
	db, err := createDatabase()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Get database statistics
	stats, err := db.Stats()
	if err != nil {
		return fmt.Errorf("failed to get database stats: %w", err)
	}

	// Display statistics
	fmt.Println("📊 Database Statistics")
	fmt.Println("=====================")
	fmt.Printf("Total Traces: %v\n", stats["total_traces"])
	fmt.Printf("Total Sessions: %v\n", stats["total_sessions"])
	fmt.Printf("Cache Entries: %v\n", stats["total_cache_entries"])

	if size, ok := stats["database_size_bytes"].(int64); ok {
		fmt.Printf("Database Size: %s\n", formatBytes(size))
	}

	return nil
}

func runDatabaseCleanup(cmd *cobra.Command, args []string) error {
	// Create database connection
	db, err := createDatabase()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create repository and cache
	repo := database.NewRepository(db)
	cache := database.NewCache(repo)

	// Clean expired cache entries
	if err := cache.CleanExpired(); err != nil {
		return fmt.Errorf("failed to clean expired cache: %w", err)
	}

	fmt.Println("✅ Expired cache entries cleaned successfully")
	return nil
}

func runDatabaseInfo(cmd *cobra.Command, args []string) error {
	// Create database connection
	db, err := createDatabase()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Get database path
	dbPath := db.GetPath()

	// Get file info
	fileInfo, err := os.Stat(dbPath)
	if err != nil {
		return fmt.Errorf("failed to get database file info: %w", err)
	}

	fmt.Println("🗄️  Database Information")
	fmt.Println("=======================")
	fmt.Printf("Location: %s\n", dbPath)
	fmt.Printf("Size: %s\n", formatBytes(fileInfo.Size()))
	fmt.Printf("Created: %s\n", fileInfo.ModTime().Format(time.RFC3339))
	fmt.Printf("Permissions: %s\n", fileInfo.Mode().String())

	return nil
}

func createDatabase() (*database.Database, error) {
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create database path
	dbPath := filepath.Join(homeDir, ".deeper", "deeper.db")

	// Create database connection
	return database.NewDatabase(dbPath)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
