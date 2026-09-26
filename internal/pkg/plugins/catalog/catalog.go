// Package catalog is the one place that knows every plugin. Plugins are
// built from the caller's config rather than self-registering from init(),
// so CLI overrides reach them and tests can assemble their own registry.
package catalog

import (
	"errors"
	"fmt"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
	academicpapers "github.com/smirnoffmg/deeper/internal/pkg/plugins/academic_papers"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/bluesky_profile"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/codeforces_profile"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/coderepos"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/companyregistry"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/contact_crawler"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/crowdin_profile"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/crtsh"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/dns_records"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/dns_resolver"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/facebook"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/github_identity"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/github_keys"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/github_profile"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/gravatar"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/habr_profile"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/ip_intel"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/keybase_profile"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/launchpad_profile"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/linuxorgru_profile"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/social_profiles"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/subdomains"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/telegram_profile"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/url_resolver"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins/whois"
)

type registrant interface {
	plugins.DeeperPlugin
	Register(r plugins.Registry) error
}

// New builds and registers every plugin. A plugin that fails to register is
// left out and its error returned alongside the usable registry: one
// unreachable source (social_profiles downloads its site list) must not
// stop a scan that the other sources can still serve.
func New(cfg *config.Config) (plugins.Registry, error) {
	all := []registrant{
		academicpapers.NewPlugin(cfg),
		bluesky_profile.NewPlugin(cfg),
		codeforces_profile.NewPlugin(cfg),
		coderepos.NewPlugin(cfg),
		companyregistry.NewPlugin(cfg),
		contact_crawler.NewPlugin(cfg),
		crowdin_profile.NewPlugin(cfg),
		crtsh.NewPlugin(cfg),
		dns_records.NewPlugin(cfg),
		dns_resolver.NewPlugin(),
		facebook.NewPlugin(cfg),
		github_identity.NewPlugin(cfg),
		github_keys.NewPlugin(cfg),
		github_profile.NewPlugin(cfg),
		gravatar.NewPlugin(cfg),
		habr_profile.NewPlugin(cfg),
		ip_intel.NewPlugin(),
		keybase_profile.NewPlugin(cfg),
		launchpad_profile.NewPlugin(cfg),
		linuxorgru_profile.NewPlugin(cfg),
		social_profiles.NewSocialProfilesPlugin(),
		subdomains.NewPlugin(cfg),
		telegram_profile.NewPlugin(cfg),
		url_resolver.NewPlugin(),
		whois.NewPlugin(),
	}

	registry := plugins.Registry{}
	var errs []error
	for _, p := range all {
		if err := p.Register(registry); err != nil {
			errs = append(errs, fmt.Errorf("register %s: %w", p, err))
		}
	}
	return registry, errors.Join(errs...)
}
