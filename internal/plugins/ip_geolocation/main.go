package ip_geolocation

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/state"
)

const InputTraceType = entities.IpAddr

func init() {
	p := NewPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type IpGeolocationPlugin struct{}

func NewPlugin() *IpGeolocationPlugin {
	return &IpGeolocationPlugin{}
}

func (p *IpGeolocationPlugin) Register() error {
	state.RegisterPlugin(InputTraceType, p)
	return nil
}

func (p *IpGeolocationPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	geolocationInfo, err := fetchGeolocation(trace.Value)
	if err != nil {
		return nil, err
	}

	geolocationJSON, err := json.Marshal(geolocationInfo)
	if err != nil {
		return nil, err
	}

	newTrace := entities.Trace{
		Value: string(geolocationJSON),
		Type:  entities.Geolocation,
	}

	return []entities.Trace{newTrace}, nil
}

func (p *IpGeolocationPlugin) String() string {
	return "IpGeolocationPlugin"
}

type GeolocationInfo struct {
	Status      string  `json:"status"`
	Message     string  `json:"message"`
	Continent   string  `json:"continent"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	Region      string  `json:"region"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
	District    string  `json:"district"`
	Zip         string  `json:"zip"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Timezone    string  `json:"timezone"`
	Offset      int     `json:"offset"`
	Currency    string  `json:"currency"`
	Isp         string  `json:"isp"`
	Org         string  `json:"org"`
	As          string  `json:"as"`
	Asname      string  `json:"asname"`
	Reverse     string  `json:"reverse"`
	Mobile      bool    `json:"mobile"`
	Proxy       bool    `json:"proxy"`
	Hosting     bool    `json:"hosting"`
	Query       string  `json:"query"`
}

var fetchGeolocation = func(ip string) (*GeolocationInfo, error) {
	url := fmt.Sprintf("http://ip-api.com/json/%s", ip)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var geolocationInfo GeolocationInfo
	if err := json.NewDecoder(resp.Body).Decode(&geolocationInfo); err != nil {
		return nil, err
	}

	if geolocationInfo.Status == "fail" {
		return nil, fmt.Errorf("geolocation lookup failed: %s", geolocationInfo.Message)
	}

	return &geolocationInfo, nil
}
