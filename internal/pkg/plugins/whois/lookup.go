package whois

import (
	"context"
	"io"
	"net"
	"strings"
	"time"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

const ianaWhoisServer = "whois.iana.org:43"

const whoisPort = "43"

type whoisClient interface {
	Query(ctx context.Context, address, term string) (string, error)
}

type tcpWhoisClient struct {
	timeout time.Duration
}

func (c *tcpWhoisClient) Query(ctx context.Context, address, term string) (string, error) {
	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(c.timeout))

	if _, err := conn.Write([]byte(term + "\r\n")); err != nil {
		return "", err
	}

	data, err := io.ReadAll(conn)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func lookupWhois(ctx context.Context, client whoisClient, domain string) ([]entities.Trace, error) {
	tld := domain[strings.LastIndex(domain, ".")+1:]

	ianaResp, err := client.Query(ctx, ianaWhoisServer, tld)
	if err != nil {
		return nil, err
	}

	referral := parseReferral(ianaResp)
	if referral == "" {
		return nil, nil
	}

	domainResp, err := client.Query(ctx, net.JoinHostPort(referral, whoisPort), domain)
	if err != nil {
		return nil, err
	}

	return parseWhoisLines(domainResp), nil
}

func parseReferral(response string) string {
	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "refer:") || strings.HasPrefix(lower, "whois:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

var companyIDKeys = map[string]bool{
	"taxpayer-id": true,
	"inn":         true,
}

func parseWhoisLines(response string) []entities.Trace {
	var traces []entities.Trace
	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "%") || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		value := strings.TrimSpace(parts[1])
		if value == "" {
			continue
		}
		traces = append(traces, entities.Trace{Value: line, Type: entities.Whois})

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		if companyIDKeys[key] {
			traces = append(traces, entities.Trace{Value: value, Type: entities.Company})
		}
	}
	return traces
}
