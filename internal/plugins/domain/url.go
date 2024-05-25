package domain

import (
	"net/http"

	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/state"
)

var protocols = []string{"http", "https"}

func init() {
	u := UrlGenerator{}
	u.Register()
}

type UrlGenerator struct {
}

func (m *UrlGenerator) Register() error {
	plugins := state.ActivePlugins[entities.Domain]
	state.ActivePlugins[entities.Domain] = append(plugins, m)
	return nil
}

func (m *UrlGenerator) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	domain := trace.Value

	result := []entities.Trace{}

	for _, protocol := range protocols {
		url := protocol + "://" + domain

		// check if url is valid
		resp, err := http.Get(url)

		if err != nil {
			continue
		}

		if resp.StatusCode != 200 {
			continue
		}

		result = append(result, entities.Trace{
			Value: url,
			Type:  entities.Url,
		})
	}

	return result, nil

}

func (m UrlGenerator) String() string {
	return "UrlGenerator"
}
