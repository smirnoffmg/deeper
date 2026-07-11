package dns_records

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDoHFetcher struct {
	responses map[string]string
	err       error
	errURLs   map[string]bool
}

func (f *fakeDoHFetcher) Get(ctx context.Context, url string) (*http.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.errURLs[url] {
		return nil, fmt.Errorf("simulated error for %s", url)
	}
	body, ok := f.responses[url]
	if !ok {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"Status":0}`)),
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func soaURL(domain string) string {
	return fmt.Sprintf("https://dns.google/resolve?name=%s&type=SOA", domain)
}

func caaURL(domain string) string {
	return fmt.Sprintf("https://dns.google/resolve?name=%s&type=CAA", domain)
}

func mxURL(domain string) string {
	return fmt.Sprintf("https://dns.google/resolve?name=%s&type=MX", domain)
}

func txtURL(domain string) string {
	return fmt.Sprintf("https://dns.google/resolve?name=%s&type=TXT", domain)
}

func nsURL(domain string) string {
	return fmt.Sprintf("https://dns.google/resolve?name=%s&type=NS", domain)
}

func cnameURL(domain string) string {
	return fmt.Sprintf("https://dns.google/resolve?name=%s&type=CNAME", domain)
}

func TestLookupDoHRecords_SOAAndCAA(t *testing.T) {
	domain := "example.com"
	fetcher := &fakeDoHFetcher{
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
	}

	traces := lookupDoHRecords(context.Background(), domain, fetcher)

	require.Len(t, traces, 3)

	soaTrace := findTrace(traces, entities.DnsRecordSOA)
	require.NotNil(t, soaTrace)
	assert.Contains(t, soaTrace.Value, "hostmaster.example.com.")

	caaTrace := findTrace(traces, entities.DnsRecordCAA)
	require.NotNil(t, caaTrace)
	assert.Equal(t, `0 issue "letsencrypt.org"`, caaTrace.Value)

	emailTrace := findTrace(traces, entities.Email)
	require.NotNil(t, emailTrace)
	assert.Equal(t, "hostmaster@example.com", emailTrace.Value)
}

func TestLookupDoHRecords_MX(t *testing.T) {
	domain := "example.com"
	fetcher := &fakeDoHFetcher{
		responses: map[string]string{
			mxURL(domain): `{
				"Status": 0,
				"Answer": [
					{"name": "example.com.", "type": 15, "data": "10 ALT4.ASPMX.L.GOOGLE.COM."},
					{"name": "example.com.", "type": 15, "data": "1 ASPMX.L.GOOGLE.COM."}
				]
			}`,
		},
	}

	traces := lookupDoHRecords(context.Background(), domain, fetcher)

	mxTraces := findAllTraces(traces, entities.DnsRecordMX)
	require.Len(t, mxTraces, 2)
	assert.Equal(t, "ALT4.ASPMX.L.GOOGLE.COM.", mxTraces[0].Value)
	assert.Equal(t, "ASPMX.L.GOOGLE.COM.", mxTraces[1].Value)
}

func TestLookupDoHRecords_TXT(t *testing.T) {
	domain := "example.com"
	fetcher := &fakeDoHFetcher{
		responses: map[string]string{
			txtURL(domain): `{
				"Status": 0,
				"Answer": [
					{"name": "example.com.", "type": 16, "data": "v=spf1 include:_spf.google.com ~all"},
					{"name": "example.com.", "type": 16, "data": "google-site-verification=abc123"}
				]
			}`,
		},
	}

	traces := lookupDoHRecords(context.Background(), domain, fetcher)

	txtTraces := findAllTraces(traces, entities.DnsRecordTXT)
	require.Len(t, txtTraces, 2)
	assert.Equal(t, "v=spf1 include:_spf.google.com ~all", txtTraces[0].Value)
	assert.Equal(t, "google-site-verification=abc123", txtTraces[1].Value)
}

func TestLookupDoHRecords_NS(t *testing.T) {
	domain := "example.com"
	fetcher := &fakeDoHFetcher{
		responses: map[string]string{
			nsURL(domain): `{
				"Status": 0,
				"Answer": [
					{"name": "example.com.", "type": 2, "data": "ns1.example.com."},
					{"name": "example.com.", "type": 2, "data": "ns2.example.com."}
				]
			}`,
		},
	}

	traces := lookupDoHRecords(context.Background(), domain, fetcher)

	nsTraces := findAllTraces(traces, entities.DnsRecordNS)
	require.Len(t, nsTraces, 2)
	assert.Equal(t, "ns1.example.com.", nsTraces[0].Value)
	assert.Equal(t, "ns2.example.com.", nsTraces[1].Value)
}

func TestLookupDoHRecords_CNAME_External(t *testing.T) {
	domain := "shop.example.com"
	fetcher := &fakeDoHFetcher{
		responses: map[string]string{
			cnameURL(domain): `{
				"Status": 0,
				"Answer": [{"name": "shop.example.com.", "type": 5, "data": "shops.myshopify.com."}]
			}`,
		},
	}

	traces := lookupDoHRecords(context.Background(), domain, fetcher)

	cnameTraces := findAllTraces(traces, entities.DnsRecordCNAME)
	require.Len(t, cnameTraces, 1)
	assert.Equal(t, "shops.myshopify.com.", cnameTraces[0].Value)
}

func TestLookupDoHRecords_CNAME_SelfReferentialIsSuppressed(t *testing.T) {
	domain := "example.com"
	fetcher := &fakeDoHFetcher{
		responses: map[string]string{
			cnameURL(domain): `{
				"Status": 0,
				"Answer": [{"name": "example.com.", "type": 5, "data": "example.com."}]
			}`,
		},
	}

	traces := lookupDoHRecords(context.Background(), domain, fetcher)

	assert.Empty(t, findAllTraces(traces, entities.DnsRecordCNAME))
}

func TestLookupDoHRecords_CNAME_NoRecordIsEmpty(t *testing.T) {
	domain := "example.com"
	fetcher := &fakeDoHFetcher{
		responses: map[string]string{
			cnameURL(domain): `{"Status": 0}`,
		},
	}

	traces := lookupDoHRecords(context.Background(), domain, fetcher)

	assert.Empty(t, findAllTraces(traces, entities.DnsRecordCNAME))
}

// TestLookupDoHRecords_OneTypeFailingDoesNotSuppressOthers is a regression
// test: each record type is an independent DoH request, so one failing
// (e.g. a transient error on the MX query) must not block the others.
func TestLookupDoHRecords_OneTypeFailingDoesNotSuppressOthers(t *testing.T) {
	domain := "example.com"
	fetcher := &fakeDoHFetcher{
		responses: map[string]string{
			nsURL(domain): `{
				"Status": 0,
				"Answer": [{"name": "example.com.", "type": 2, "data": "ns1.example.com."}]
			}`,
		},
		errURLs: map[string]bool{
			mxURL(domain): true,
		},
	}

	traces := lookupDoHRecords(context.Background(), domain, fetcher)

	assert.NotEmpty(t, findAllTraces(traces, entities.DnsRecordNS))
	assert.Empty(t, findAllTraces(traces, entities.DnsRecordMX))
}

func findAllTraces(traces []entities.Trace, typ entities.TraceType) []entities.Trace {
	var out []entities.Trace
	for _, tr := range traces {
		if tr.Type == typ {
			out = append(out, tr)
		}
	}
	return out
}

func TestDecodeSOARnameEmail(t *testing.T) {
	tests := []struct {
		name   string
		rname  string
		want   string
		wantOK bool
	}{
		{
			name:   "plain RNAME",
			rname:  "hostmaster.example.com.",
			want:   "hostmaster@example.com",
			wantOK: true,
		},
		{
			name:   "escaped dot in local part",
			rname:  `john\.doe.example.com.`,
			want:   "john.doe@example.com",
			wantOK: true,
		},
		{
			name:   "malformed no separator",
			rname:  "hostmaster",
			wantOK: false,
		},
		{
			name:   "empty",
			rname:  "",
			wantOK: false,
		},
		{
			name:   "domain only after separator",
			rname:  ".example.com.",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := decodeSOARnameEmail(tt.rname)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestLookupDoHRecords_MalformedSOA(t *testing.T) {
	domain := "example.com"
	fetcher := &fakeDoHFetcher{
		responses: map[string]string{
			soaURL(domain): `{
				"Status": 0,
				"Answer": [{
					"name": "example.com.",
					"type": 6,
					"data": "incomplete"
				}]
			}`,
			caaURL(domain): `{"Status": 0}`,
		},
	}

	traces := lookupDoHRecords(context.Background(), domain, fetcher)

	require.Len(t, traces, 1)
	assert.Equal(t, entities.DnsRecordSOA, traces[0].Type)
	assert.Equal(t, "incomplete", traces[0].Value)
	assert.Nil(t, findTrace(traces, entities.Email))
}

func TestLookupDoHRecords_FetchError(t *testing.T) {
	fetcher := &fakeDoHFetcher{err: fmt.Errorf("network error")}

	traces := lookupDoHRecords(context.Background(), "example.com", fetcher)

	assert.Empty(t, traces)
}

func findTrace(traces []entities.Trace, typ entities.TraceType) *entities.Trace {
	for i := range traces {
		if traces[i].Type == typ {
			return &traces[i]
		}
	}
	return nil
}
