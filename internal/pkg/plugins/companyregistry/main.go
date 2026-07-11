package companyregistry

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
)

func init() {
	p := NewPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type CompanyRegistryPlugin struct {
	fetcher searchFetcher
}

func NewPlugin() *CompanyRegistryPlugin {
	return &CompanyRegistryPlugin{fetcher: deeperhttp.NewClient(config.LoadConfig())}
}

func (p *CompanyRegistryPlugin) Register() error {
	state.RegisterPlugin(entities.Company, p)
	return nil
}

func (p *CompanyRegistryPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Company {
		return nil, nil
	}
	return searchCompany(context.Background(), p.fetcher, trace.Value)
}

func (p CompanyRegistryPlugin) String() string {
	return "CompanyRegistryPlugin"
}
