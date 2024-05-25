package username

import (
	"context"
	"os"

	"github.com/google/go-github/v62/github"
	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/state"
)

func init() {
	g := NewGithubPlugin()
	g.Register()
}

type GithubPlugin struct {
	client *github.Client
}

func NewGithubPlugin() *GithubPlugin {
	token := os.Getenv("GITHUB_TOKEN")

	if token == "" {
		log.Warn().Msg("GITHUB_TOKEN not found in env")
		return &GithubPlugin{
			client: github.NewClient(nil),
		}
	}

	return &GithubPlugin{
		client: github.NewClient(nil).WithAuthToken(token),
	}

}

func (g *GithubPlugin) Register() error {
	plugins := state.ActivePlugins[entities.Username]
	state.ActivePlugins[entities.Username] = append(plugins, g)

	return nil
}

func (g *GithubPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	// let's check github username

	user, _, err := g.client.Users.Get(context.Background(), trace.Value)
	if err != nil {
		return nil, nil
	}

	traces := []entities.Trace{
		{
			Value: user.GetHTMLURL(),
			Type:  entities.Github,
		},
		{
			Value: user.GetEmail(),
			Type:  entities.Email,
		},
		{
			Value: user.GetLocation(),
			Type:  entities.Address,
		},
		{
			Value: user.GetName(),
			Type:  entities.Name,
		},
		{
			Value: user.GetCompany(),
			Type:  entities.Company,
		},
		{
			Value: user.GetTwitterUsername(),
			Type:  entities.Twitter,
		},
		{
			Value: user.GetBlog(),
			Type:  entities.Url,
		},
	}

	// get all repositories
	repos, _, err := g.client.Repositories.ListByUser(context.Background(), trace.Value, nil)

	if err != nil {
		return traces, nil
	}

	for _, repo := range repos {
		traces = append(traces, entities.Trace{
			Value: repo.GetHTMLURL(),
			Type:  entities.Repository,
		})
	}

	return traces, nil

}

func (g GithubPlugin) String() string {
	return "GithubPlugin"
}
