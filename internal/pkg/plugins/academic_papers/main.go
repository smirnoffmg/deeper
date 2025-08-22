package academicpapers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
	"github.com/texttheater/golang-levenshtein/levenshtein"
)

const InputTraceType = entities.Username

func init() {
	p := NewPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type AcademicPapersPlugin struct{}

func NewPlugin() *AcademicPapersPlugin {
	return &AcademicPapersPlugin{}
}

func (g *AcademicPapersPlugin) Register() error {
	state.RegisterPlugin(InputTraceType, g)
	return nil
}

type Paper struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type SearchResult struct {
	Data []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Authors []struct {
			Name string `json:"name"`
		} `json:"authors"`
	} `json:"data"`
}

func (g *AcademicPapersPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	nameQuery := strings.ReplaceAll(trace.Value, " ", "%20")
	url := fmt.Sprintf("https://api.semanticscholar.org/graph/v1/author/search?query=%s", nameQuery)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var newTraces []entities.Trace
	for _, paper := range result.Data {
		for _, author := range paper.Authors {
			distance := levenshtein.DistanceForStrings([]rune(trace.Value), []rune(author.Name), levenshtein.DefaultOptions)
			if distance <= len(trace.Value)/2 {
				newTraces = append(newTraces, entities.Trace{
					Value: paper.URL,
					Type:  entities.Url,
				})
			}
		}
	}

	return newTraces, nil
}

func (g AcademicPapersPlugin) String() string {
	return "AcademicPapersPlugin"
}
