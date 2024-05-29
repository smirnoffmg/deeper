package domain

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"

	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/state"
)

type SubdomainPlugin struct{}

func init() {
	plugin := SubdomainPlugin{}
	plugin.Register()
}

func (p SubdomainPlugin) Register() error {
	state.RegisterPlugin(entities.Domain, p)
	return nil
}

func (p SubdomainPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Domain {
		return nil, nil
	}

	url := fmt.Sprintf("https://api.hackertarget.com/hostsearch/?q=%s", trace.Value)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var newTraces []entities.Trace
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ",")
		if len(parts) == 2 {
			subdomain := parts[0]
			ipAddr := parts[1]
			newTraces = append(newTraces, entities.Trace{
				Value: subdomain,
				Type:  entities.Subdomain,
			})
			newTraces = append(newTraces, entities.Trace{
				Value: ipAddr,
				Type:  entities.IpAddr,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return newTraces, nil
}

func (p SubdomainPlugin) String() string {
	return "SubdomainPlugin"
}
