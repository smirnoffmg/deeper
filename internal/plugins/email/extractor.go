package plugins

import (
	"strings"

	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/state"
)

func init() {
	u := UsernameExtractor{}
	u.Register()

	d := DomainExtractor{}
	d.Register()
}

type UsernameExtractor struct {
}

func (m *UsernameExtractor) Register() error {

	plugins := state.ActivePlugins[entities.Email]
	state.ActivePlugins[entities.Email] = append(plugins, m)
	return nil
}

func (m *UsernameExtractor) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	username := trace.Value[:strings.Index(trace.Value, "@")]

	return []entities.Trace{
		{
			Value: username,
			Type:  entities.Username,
		},
	}, nil
}

func (m UsernameExtractor) String() string {
	return "UsernameExtractor"
}

type DomainExtractor struct {
}

func (m *DomainExtractor) Register() error {

	plugins := state.ActivePlugins[entities.Email]
	state.ActivePlugins[entities.Email] = append(plugins, m)
	return nil
}

func (m *DomainExtractor) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	domain := trace.Value[strings.Index(trace.Value, "@")+1:]

	return []entities.Trace{
		{
			Value: domain,
			Type:  entities.Domain,
		},
	}, nil
}

func (m DomainExtractor) String() string {
	return "DomainExtractor"
}
