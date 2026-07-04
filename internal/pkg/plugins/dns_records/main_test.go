package dns_records

import (
	"net"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDNSRecordsPlugin_FollowTrace_WrongType(t *testing.T) {
	plugin := &DNSRecordsPlugin{
		lookups: &fakeDNSLookups{},
		doh:     &fakeDoHFetcher{},
	}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "1.2.3.4", Type: entities.IpAddr})

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestDNSRecordsPlugin_FollowTrace_SkipsWildcard(t *testing.T) {
	plugin := &DNSRecordsPlugin{
		lookups: &fakeDNSLookups{
			mx: []*net.MX{{Host: "mail.example.com.", Pref: 10}},
		},
		doh: &fakeDoHFetcher{},
	}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "*.example.com", Type: entities.Domain})

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestDNSRecordsPlugin_FollowTrace_AllRecordTypes(t *testing.T) {
	domain := "example.com"
	plugin := &DNSRecordsPlugin{
		lookups: &fakeDNSLookups{
			mx:    []*net.MX{{Host: "mail.example.com.", Pref: 10}},
			txt:   []string{"v=spf1 ~all"},
			ns:    []*net.NS{{Host: "ns.example.com."}},
			cname: "cdn.example.net.",
		},
		doh: &fakeDoHFetcher{
			responses: map[string]string{
				soaURL(domain): `{
					"Status": 0,
					"Answer": [{
						"name": "example.com.",
						"type": 6,
						"data": "ns1.example.com. hostmaster.example.com. 2024010100 7200 3600 1209600 3600"
					}]
				}`,
				caaURL(domain): `{
					"Status": 0,
					"Answer": [{
						"name": "example.com.",
						"type": 257,
						"data": "0 issue \"letsencrypt.org\""
					}]
				}`,
			},
		},
	}

	traces, err := plugin.FollowTrace(entities.Trace{Value: domain, Type: entities.Domain})

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(traces), 6)

	types := traceTypes(traces)
	assert.Contains(t, types, entities.DnsRecordMX)
	assert.Contains(t, types, entities.DnsRecordTXT)
	assert.Contains(t, types, entities.DnsRecordNS)
	assert.Contains(t, types, entities.DnsRecordCNAME)
	assert.Contains(t, types, entities.DnsRecordSOA)
	assert.Contains(t, types, entities.DnsRecordCAA)
	assert.Contains(t, types, entities.Email)
}

// TestDNSRecordsPlugin_FollowTrace_StdlibFailureStillReturnsDoH is a
// regression test for a bug found live against codescoring.ru: FollowTrace
// used to return early (losing all results) whenever any stdlib lookup
// (MX/TXT/NS/CNAME) errored, even though the independent DoH-backed SOA/CAA
// lookups had already succeeded or would have succeeded. Verified externally
// that Google's DoH endpoint resolves fine in environments where raw UDP
// DNS-53 queries are refused.
func TestDNSRecordsPlugin_FollowTrace_StdlibFailureStillReturnsDoH(t *testing.T) {
	domain := "example.com"
	plugin := &DNSRecordsPlugin{
		lookups: &fakeDNSLookups{
			mxErr:    net.UnknownNetworkError("connection refused"),
			txtErr:   net.UnknownNetworkError("connection refused"),
			nsErr:    net.UnknownNetworkError("connection refused"),
			cnameErr: net.UnknownNetworkError("connection refused"),
		},
		doh: &fakeDoHFetcher{
			responses: map[string]string{
				soaURL(domain): `{
					"Status": 0,
					"Answer": [{
						"name": "example.com.",
						"type": 6,
						"data": "ns1.example.com. hostmaster.example.com. 2024010100 7200 3600 1209600 3600"
					}]
				}`,
			},
		},
	}

	traces, err := plugin.FollowTrace(entities.Trace{Value: domain, Type: entities.Domain})

	require.NoError(t, err)
	require.NotEmpty(t, traces)

	types := traceTypes(traces)
	assert.Contains(t, types, entities.DnsRecordSOA)
	assert.Contains(t, types, entities.Email)
	assert.NotContains(t, types, entities.DnsRecordMX)
}

func TestDNSRecordsPlugin_FollowTrace_Subdomain(t *testing.T) {
	plugin := &DNSRecordsPlugin{
		lookups: &fakeDNSLookups{
			txt: []string{"verified"},
		},
		doh: &fakeDoHFetcher{},
	}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "www.example.com", Type: entities.Subdomain})

	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.DnsRecordTXT, traces[0].Type)
}

func TestDNSRecordsPlugin_String(t *testing.T) {
	plugin := NewPlugin()
	assert.Equal(t, "DNSRecordsPlugin", plugin.String())
}

func traceTypes(traces []entities.Trace) []entities.TraceType {
	types := make([]entities.TraceType, len(traces))
	for i, tr := range traces {
		types[i] = tr.Type
	}
	return types
}

var _ dohFetcher = (*fakeDoHFetcher)(nil)
var _ dnsLookups = (*fakeDNSLookups)(nil)
