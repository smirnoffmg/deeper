package main

import (
	"log"

	"github.com/smirnoffmg/deeper/internal/app/deeper"

	// Import plugins to register them
	_ "github.com/smirnoffmg/deeper/internal/pkg/plugins/academic_papers"
	_ "github.com/smirnoffmg/deeper/internal/pkg/plugins/contact_crawler"
	_ "github.com/smirnoffmg/deeper/internal/pkg/plugins/coderepos"
	_ "github.com/smirnoffmg/deeper/internal/pkg/plugins/crtsh"
	_ "github.com/smirnoffmg/deeper/internal/pkg/plugins/dns_resolver"
	_ "github.com/smirnoffmg/deeper/internal/pkg/plugins/facebook"
	_ "github.com/smirnoffmg/deeper/internal/pkg/plugins/social_profiles"
	_ "github.com/smirnoffmg/deeper/internal/pkg/plugins/subdomains"
)

func main() {
	// Create and run the application with uber-fx
	app := deeper.NewApp()

	// Start the application lifecycle
	if err := app.Run(); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}
