package ip_intel

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTXTLookup struct {
	responses map[string][]string
	errs      map[string]error
}

func (f *fakeTXTLookup) LookupTXT(ctx context.Context, name string) ([]string, error) {
	if err, ok := f.errs[name]; ok {
		return nil, err
	}
	if records, ok := f.responses[name]; ok {
		return records, nil
	}
	return nil, nil
}

func TestLookupASN_KnownResponse(t *testing.T) {
	ip := "198.51.100.5"
	lookups := &fakeTXTLookup{
		responses: map[string][]string{
			"5.100.51.198.origin.asn.cymru.com": {
				"24940 | 198.51.100.0/24 | DE | ripencc | 2003-03-17",
			},
			"AS24940.asn.cymru.com": {
				"24940 | DE | ripencc | 2003-03-17 | HETZNER-AS, DE",
			},
		},
	}

	traces := lookupASN(context.Background(), ip, lookups)

	require.Len(t, traces, 3)

	assert.Equal(t, entities.ASN, traces[0].Type)
	assert.Equal(t, "AS24940", traces[0].Value)
	assert.Equal(t, entities.Netblock, traces[1].Type)
	assert.Equal(t, "198.51.100.0/24", traces[1].Value)
	assert.Equal(t, entities.Company, traces[2].Type)
	assert.Equal(t, "HETZNER-AS, DE", traces[2].Value)
}

func TestLookupASN_MalformedOrigin(t *testing.T) {
	lookups := &fakeTXTLookup{
		responses: map[string][]string{
			"5.100.51.198.origin.asn.cymru.com": {"invalid"},
		},
	}

	traces := lookupASN(context.Background(), "198.51.100.5", lookups)

	assert.Empty(t, traces)
}

func TestLookupASN_EmptyTXT(t *testing.T) {
	lookups := &fakeTXTLookup{
		responses: map[string][]string{
			"5.100.51.198.origin.asn.cymru.com": {},
		},
	}

	traces := lookupASN(context.Background(), "198.51.100.5", lookups)

	assert.Empty(t, traces)
}

func TestLookupASN_ChainedLookupFails(t *testing.T) {
	ip := "198.51.100.5"
	lookups := &fakeTXTLookup{
		responses: map[string][]string{
			"5.100.51.198.origin.asn.cymru.com": {
				"24940 | 198.51.100.0/24 | DE | ripencc | 2003-03-17",
			},
		},
		errs: map[string]error{
			"AS24940.asn.cymru.com": errors.New("lookup failed"),
		},
	}

	traces := lookupASN(context.Background(), ip, lookups)

	require.Len(t, traces, 2)
	assert.Equal(t, entities.ASN, traces[0].Type)
	assert.Equal(t, entities.Netblock, traces[1].Type)
}

func TestLookupASN_NonIPv4(t *testing.T) {
	lookups := &fakeTXTLookup{}

	traces := lookupASN(context.Background(), "2001:db8::1", lookups)

	assert.Empty(t, traces)
}

func TestLookupASN_InvalidIP(t *testing.T) {
	lookups := &fakeTXTLookup{}

	traces := lookupASN(context.Background(), "not-an-ip", lookups)

	assert.Empty(t, traces)
}

func TestReverseIPv4(t *testing.T) {
	assert.Equal(t, "5.100.51.198", reverseIPv4("198.51.100.5"))
}

func TestParseOriginResponse(t *testing.T) {
	asn, netblock, ok := parseOriginResponse("24940 | 198.51.100.0/24 | DE | ripencc | 2003-03-17")
	require.True(t, ok)
	assert.Equal(t, "24940", asn)
	assert.Equal(t, "198.51.100.0/24", netblock)
}

func TestParseASNameResponse(t *testing.T) {
	name, ok := parseASNameResponse("24940 | DE | ripencc | 2003-03-17 | HETZNER-AS, DE")
	require.True(t, ok)
	assert.Equal(t, "HETZNER-AS, DE", name)
}

func TestLookupASN_OriginLookupError(t *testing.T) {
	lookups := &fakeTXTLookup{
		errs: map[string]error{
			"5.100.51.198.origin.asn.cymru.com": fmt.Errorf("dns error"),
		},
	}

	traces := lookupASN(context.Background(), "198.51.100.5", lookups)

	assert.Empty(t, traces)
}
