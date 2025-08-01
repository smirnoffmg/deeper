package whois

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/state"
)

const InputTraceType = entities.Domain

func init() {
	p := NewPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type WhoisPlugin struct{}

func NewPlugin() *WhoisPlugin {
	return &WhoisPlugin{}
}

func (p *WhoisPlugin) Register() error {
	state.RegisterPlugin(InputTraceType, p)
	return nil
}

func (p *WhoisPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	apiKey := os.Getenv("IP2WHOIS_API_KEY")
	if apiKey == "" {
		log.Warn().Msg("IP2WHOIS_API_KEY environment variable not set. Skipping whois check.")
		return nil, nil
	}

	whoisInfo, err := fetchWhois(trace.Value, apiKey)
	if err != nil {
		return nil, err
	}

	whoisJSON, err := json.Marshal(whoisInfo)
	if err != nil {
		return nil, err
	}

	newTrace := entities.Trace{
		Value: string(whoisJSON),
		Type:  entities.Whois,
	}

	return []entities.Trace{newTrace}, nil
}

func (p *WhoisPlugin) String() string {
	return "WhoisPlugin"
}

type WhoisInfo struct {
	Domain     string `json:"domain"`
	DomainId   string `json:"domain_id"`
	Status     string `json:"status"`
	CreateDate string `json:"create_date"`
	UpdateDate string `json:"update_date"`
	ExpireDate string `json:"expire_date"`
	DomainAge  int    `json:"domain_age"`
	WhoisServer string `json:"whois_server"`
	Registrar  struct {
		IanaId string `json:"iana_id"`
		Name   string `json:"name"`
		Url    string `json:"url"`
	} `json:"registrar"`
	Registrant struct {
		Name          string `json:"name"`
		Organization  string `json:"organization"`
		StreetAddress string `json:"street_address"`
		City          string `json:"city"`
		Region        string `json:"region"`
		ZipCode       string `json:"zip_code"`
		Country       string `json:"country"`
		Phone         string `json:"phone"`
		Fax           string `json:"fax"`
		Email         string `json:"email"`
	} `json:"registrant"`
	Admin struct {
		Name          string `json:"name"`
		Organization  string `json:"organization"`
		StreetAddress string `json:"street_address"`
		City          string `json:"city"`
		Region        string `json:"region"`
		ZipCode       string `json:"zip_code"`
		Country       string `json:"country"`
		Phone         string `json:"phone"`
		Fax           string `json:"fax"`
		Email         string `json:"email"`
	} `json:"admin"`
	Tech struct {
		Name          string `json:"name"`
		Organization  string `json:"organization"`
		StreetAddress string `json:"street_address"`
		City          string `json:"city"`
		Region        string `json:"region"`
		ZipCode       string `json:"zip_code"`
		Country       string `json:"country"`
		Phone         string `json:"phone"`
		Fax           string `json:"fax"`
		Email         string `json:"email"`
	} `json:"tech"`
	Billing struct {
		Name          string `json:"name"`
		Organization  string `json:"organization"`
		StreetAddress string `json:"street_address"`
		City          string `json:"city"`
		Region        string `json:"region"`
		ZipCode       string `json:"zip_code"`
		Country       string `json:"country"`
		Phone         string `json:"phone"`
		Fax           string `json:"fax"`
		Email         string `json:"email"`
	} `json:"billing"`
	Nameservers []string `json:"nameservers"`
}

var fetchWhois = func(domain string, apiKey string) (*WhoisInfo, error) {
	url := fmt.Sprintf("https://api.ip2whois.com/v2?key=%s&domain=%s", apiKey, domain)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var whoisInfo WhoisInfo
	if err := json.NewDecoder(resp.Body).Decode(&whoisInfo); err != nil {
		return nil, err
	}

	return &whoisInfo, nil
}
