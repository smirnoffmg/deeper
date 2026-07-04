package dns_records

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDNSLookups struct {
	mx    []*net.MX
	mxErr error

	txt    []string
	txtErr error

	ns    []*net.NS
	nsErr error

	cname    string
	cnameErr error
}

func (f *fakeDNSLookups) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	return f.mx, f.mxErr
}

func (f *fakeDNSLookups) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return f.txt, f.txtErr
}

func (f *fakeDNSLookups) LookupNS(ctx context.Context, name string) ([]*net.NS, error) {
	return f.ns, f.nsErr
}

func (f *fakeDNSLookups) LookupCNAME(ctx context.Context, host string) (string, error) {
	return f.cname, f.cnameErr
}

func TestLookupStdlibRecords_MX(t *testing.T) {
	lookups := &fakeDNSLookups{
		mx: []*net.MX{
			{Host: "mail1.example.com.", Pref: 10},
			{Host: "mail2.example.com.", Pref: 20},
		},
	}

	traces := lookupStdlibRecords(context.Background(), "example.com", lookups)

	require.Len(t, traces, 2)
	assert.Equal(t, entities.DnsRecordMX, traces[0].Type)
	assert.Equal(t, "mail1.example.com.", traces[0].Value)
	assert.Equal(t, entities.DnsRecordMX, traces[1].Type)
	assert.Equal(t, "mail2.example.com.", traces[1].Value)
}

func TestLookupStdlibRecords_TXT(t *testing.T) {
	lookups := &fakeDNSLookups{
		txt: []string{
			"v=spf1 include:_spf.google.com ~all",
			"google-site-verification=abc123",
		},
	}

	traces := lookupStdlibRecords(context.Background(), "example.com", lookups)

	require.Len(t, traces, 2)
	assert.Equal(t, entities.DnsRecordTXT, traces[0].Type)
	assert.Equal(t, "v=spf1 include:_spf.google.com ~all", traces[0].Value)
	assert.Equal(t, entities.DnsRecordTXT, traces[1].Type)
	assert.Equal(t, "google-site-verification=abc123", traces[1].Value)
}

func TestLookupStdlibRecords_NS(t *testing.T) {
	lookups := &fakeDNSLookups{
		ns: []*net.NS{
			{Host: "ns1.example.com."},
			{Host: "ns2.example.com."},
		},
	}

	traces := lookupStdlibRecords(context.Background(), "example.com", lookups)

	require.Len(t, traces, 2)
	assert.Equal(t, entities.DnsRecordNS, traces[0].Type)
	assert.Equal(t, "ns1.example.com.", traces[0].Value)
	assert.Equal(t, entities.DnsRecordNS, traces[1].Type)
	assert.Equal(t, "ns2.example.com.", traces[1].Value)
}

func TestLookupStdlibRecords_CNAME_SelfReferential(t *testing.T) {
	lookups := &fakeDNSLookups{
		cname: "example.com.",
	}

	traces := lookupStdlibRecords(context.Background(), "example.com", lookups)

	assert.Empty(t, traces)
}

func TestLookupStdlibRecords_CNAME_External(t *testing.T) {
	lookups := &fakeDNSLookups{
		cname: "shops.myshopify.com.",
	}

	traces := lookupStdlibRecords(context.Background(), "shop.example.com", lookups)

	require.Len(t, traces, 1)
	assert.Equal(t, entities.DnsRecordCNAME, traces[0].Type)
	assert.Equal(t, "shops.myshopify.com.", traces[0].Value)
}

// TestLookupStdlibRecords_PartialFailure is a regression test: confirmed live
// against codescoring.ru that some environments can resolve A/AAAA/PTR via the
// OS-native resolver but reject raw MX/TXT/NS/CNAME wire queries entirely
// (connection refused to the configured nameserver). One record type failing
// must not suppress the others.
func TestLookupStdlibRecords_PartialFailure(t *testing.T) {
	lookups := &fakeDNSLookups{
		mxErr: errors.New("dns failure"),
		txt:   []string{"v=spf1 ~all"},
		nsErr: errors.New("dns failure"),
		cname: "cdn.example.net.",
	}

	traces := lookupStdlibRecords(context.Background(), "example.com", lookups)

	require.Len(t, traces, 2)
	types := make([]entities.TraceType, len(traces))
	for i, tr := range traces {
		types[i] = tr.Type
	}
	assert.Contains(t, types, entities.DnsRecordTXT)
	assert.Contains(t, types, entities.DnsRecordCNAME)
	assert.NotContains(t, types, entities.DnsRecordMX)
	assert.NotContains(t, types, entities.DnsRecordNS)
}

func TestLookupStdlibRecords_AllLookupsFail(t *testing.T) {
	lookups := &fakeDNSLookups{
		mxErr:    errors.New("dns failure"),
		txtErr:   errors.New("dns failure"),
		nsErr:    errors.New("dns failure"),
		cnameErr: errors.New("dns failure"),
	}

	traces := lookupStdlibRecords(context.Background(), "example.com", lookups)

	assert.Empty(t, traces)
}

func TestLookupStdlibRecords_AllTypes(t *testing.T) {
	lookups := &fakeDNSLookups{
		mx:    []*net.MX{{Host: "mail.example.com.", Pref: 10}},
		txt:   []string{"v=spf1 ~all"},
		ns:    []*net.NS{{Host: "ns.example.com."}},
		cname: "cdn.example.net.",
	}

	traces := lookupStdlibRecords(context.Background(), "example.com", lookups)

	require.Len(t, traces, 4)

	types := make([]entities.TraceType, len(traces))
	for i, tr := range traces {
		types[i] = tr.Type
	}
	assert.Contains(t, types, entities.DnsRecordMX)
	assert.Contains(t, types, entities.DnsRecordTXT)
	assert.Contains(t, types, entities.DnsRecordNS)
	assert.Contains(t, types, entities.DnsRecordCNAME)
}
