package companyregistry

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type CompanyRegistryPlugin struct {
	fetcher searchFetcher
}

func NewPlugin(cfg *config.Config) *CompanyRegistryPlugin {
	return &CompanyRegistryPlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (p *CompanyRegistryPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Company, p)
	return nil
}

func (p *CompanyRegistryPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Company {
		return nil, nil
	}
	return searchCompany(ctx, p.fetcher, trace.Value)
}

func (p CompanyRegistryPlugin) String() string {
	return "CompanyRegistryPlugin"
}
