package dns_records

import (
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDNSRecordsPlugin_FollowTrace_WrongType(t *testing.T) {
	plugin := &DNSRecordsPlugin{doh: &fakeDoHFetcher{}}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "1.2.3.4", Type: entities.IpAddr})

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestDNSRecordsPlugin_FollowTrace_SkipsWildcard(t *testing.T) {
	plugin := &DNSRecordsPlugin{
		doh: &fakeDoHFetcher{
			responses: map[string]string{
				mxURL("*.example.com"): `{"Status":0,"Answer":[{"name":"*.example.com.","type":15,"data":"10 mail.example.com."}]}`,
			},
		},
	}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "*.example.com", Type: entities.Domain})

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestDNSRecordsPlugin_FollowTrace_AllRecordTypes(t *testing.T) {
	domain := "example.com"
	plugin := &DNSRecordsPlugin{
		doh: &fakeDoHFetcher{
			responses: map[string]string{
				mxURL(domain):    `{"Status":0,"Answer":[{"name":"example.com.","type":15,"data":"10 mail.example.com."}]}`,
				txtURL(domain):   `{"Status":0,"Answer":[{"name":"example.com.","type":16,"data":"v=spf1 ~all"}]}`,
				nsURL(domain):    `{"Status":0,"Answer":[{"name":"example.com.","type":2,"data":"ns.example.com."}]}`,
				cnameURL(domain): `{"Status":0}`,
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
	assert.Contains(t, types, entities.DnsRecordSOA)
	assert.Contains(t, types, entities.DnsRecordCAA)
	assert.Contains(t, types, entities.Email)
}

// TestDNSRecordsPlugin_FollowTrace_PartialDoHFailureStillReturnsOthers is a
// regression test for a bug found live against codescoring.ru: FollowTrace
// used to return early (losing all results) whenever any lookup errored. All
// six record types are now independent DoH requests — this test simulates
// an environment where raw UDP DNS-53 queries are refused (which used to
// take out MX/TXT/NS/CNAME entirely, back when they went through stdlib) by
// failing one DoH request and confirming the rest still come through.
func TestDNSRecordsPlugin_FollowTrace_PartialDoHFailureStillReturnsOthers(t *testing.T) {
	domain := "example.com"
	plugin := &DNSRecordsPlugin{
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
			errURLs: map[string]bool{
				mxURL(domain):    true,
				txtURL(domain):   true,
				nsURL(domain):    true,
				cnameURL(domain): true,
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
	domain := "www.example.com"
	plugin := &DNSRecordsPlugin{
		doh: &fakeDoHFetcher{
			responses: map[string]string{
				txtURL(domain): `{"Status":0,"Answer":[{"name":"www.example.com.","type":16,"data":"verified"}]}`,
			},
		},
	}

	traces, err := plugin.FollowTrace(entities.Trace{Value: domain, Type: entities.Subdomain})

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
