package academicpapers

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type AcademicPapersPlugin struct {
	fetcher searchFetcher
}

func NewPlugin(cfg *config.Config) *AcademicPapersPlugin {
	return &AcademicPapersPlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (g *AcademicPapersPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Username, g)
	r.Add(entities.Name, g)
	return nil
}

func (g *AcademicPapersPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Username && trace.Type != entities.Name {
		return nil, nil
	}

	urls, err := searchAuthorPapers(ctx, g.fetcher, trace.Value)
	if err != nil {
		return nil, err
	}

	var newTraces []entities.Trace
	for _, u := range urls {
		newTraces = append(newTraces, entities.Trace{Value: u, Type: entities.Url})
	}
	return newTraces, nil
}

func (g AcademicPapersPlugin) String() string {
	return "AcademicPapersPlugin"
}
