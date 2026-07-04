package ip_intel

import (
	"errors"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIPIntelPlugin_FollowTrace_WrongType(t *testing.T) {
	plugin := &IPIntelPlugin{
		txt:  &fakeTXTLookup{},
		addr: &fakeAddrLookup{},
	}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "example.com", Type: entities.Domain})

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestIPIntelPlugin_FollowTrace_FullIntel(t *testing.T) {
	ip := "198.51.100.5"
	plugin := &IPIntelPlugin{
		txt: &fakeTXTLookup{
			responses: map[string][]string{
				"5.100.51.198.origin.asn.cymru.com": {
					"24940 | 198.51.100.0/24 | DE | ripencc | 2003-03-17",
				},
				"AS24940.asn.cymru.com": {
					"24940 | DE | ripencc | 2003-03-17 | HETZNER-AS, DE",
				},
			},
		},
		addr: &fakeAddrLookup{
			names: []string{"static.198-51-100-5.clients.your-server.de."},
		},
	}

	traces, err := plugin.FollowTrace(entities.Trace{Value: ip, Type: entities.IpAddr})

	require.NoError(t, err)
	require.Len(t, traces, 4)

	types := make([]entities.TraceType, len(traces))
	for i, tr := range traces {
		types[i] = tr.Type
	}
	assert.Contains(t, types, entities.ASN)
	assert.Contains(t, types, entities.Netblock)
	assert.Contains(t, types, entities.Company)
	assert.Contains(t, types, entities.DnsRecordPTR)
}

func TestIPIntelPlugin_FollowTrace_PTROnly(t *testing.T) {
	plugin := &IPIntelPlugin{
		txt: &fakeTXTLookup{},
		addr: &fakeAddrLookup{
			names: []string{"host.example.com."},
		},
	}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "198.51.100.5", Type: entities.IpAddr})

	require.NoError(t, err)
	require.Len(t, traces, 1)
	assert.Equal(t, entities.DnsRecordPTR, traces[0].Type)
}

func TestIPIntelPlugin_FollowTrace_NoResults(t *testing.T) {
	plugin := &IPIntelPlugin{
		txt:  &fakeTXTLookup{},
		addr: &fakeAddrLookup{err: errors.New("no ptr")},
	}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "198.51.100.5", Type: entities.IpAddr})

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestIPIntelPlugin_String(t *testing.T) {
	plugin := NewPlugin()
	assert.Equal(t, "IPIntelPlugin", plugin.String())
}

var _ txtLookup = (*fakeTXTLookup)(nil)
var _ addrLookup = (*fakeAddrLookup)(nil)
