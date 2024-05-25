package domain

import (
	"net"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/entities"
	"github.com/smirnoffmg/deeper/internal/state"
)

var DomainsBlacklist = []string{
	"gmail.com",
	"yahoo.com",
	"hotmail.com",
	"outlook.com",
	"icloud.com",
	"protonmail.com",
	"mail.com",
	"zoho.com",
	"yandex.com",
	"yandex.ru",
	"yandex-team.ru",
}

func init() {
	d := DNSLookup{}
	d.Register()
}

type DNSLookup struct {
}

func (m *DNSLookup) Register() error {

	plugins := state.ActivePlugins[entities.Domain]
	state.ActivePlugins[entities.Domain] = append(plugins, m)
	return nil
}

func (m *DNSLookup) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	domain := trace.Value

	// check if domain is in blacklist
	for _, blacklistedDomain := range DomainsBlacklist {
		if domain == blacklistedDomain {
			return nil, nil
		}
	}

	result := []entities.Trace{}

	// get A and AAAA records
	records, err := net.LookupHost(domain)

	if err != nil {
		log.Error().Err(err).Msg("error looking up domain")
	}

	for _, record := range records {
		result = append(result, entities.Trace{
			Value: record,
			Type:  entities.IpAddr,
		})
	}

	// get MX records
	mxRecords, err := net.LookupMX(domain)

	if err != nil {
		log.Error().Err(err).Msg("error looking up domain")
	}

	for _, mxRecord := range mxRecords {
		result = append(result, entities.Trace{
			Value: mxRecord.Host,
			Type:  entities.DnsRecordMX,
		})
	}

	// get NS records

	nsRecords, err := net.LookupNS(domain)

	if err != nil {
		log.Error().Err(err).Msg("error looking up domain")
	}

	for _, nsRecord := range nsRecords {
		result = append(result, entities.Trace{
			Value: nsRecord.Host,
			Type:  entities.DnsRecordNS,
		})
	}

	// get TXT records

	txtRecords, err := net.LookupTXT(domain)

	if err != nil {
		log.Error().Err(err).Msg("error looking up domain")
	}

	for _, txtRecord := range txtRecords {
		result = append(result, entities.Trace{
			Value: txtRecord,
			Type:  entities.DnsRecordTXT,
		})
	}

	// get CNAME records

	cnameRecords, err := net.LookupCNAME(domain)

	if err != nil {
		log.Error().Err(err).Msg("error looking up domain")
	}

	result = append(result, entities.Trace{
		Value: cnameRecords,
		Type:  entities.DnsRecordCNAME,
	})

	return result, nil
}

func (m DNSLookup) String() string {
	return "DNSLookup"
}
