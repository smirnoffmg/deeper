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
}

func (f *fakeDoHFetcher) Get(ctx context.Context, url string) (*http.Response, error) {
	if f.err != nil {
		return nil, f.err
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

func TestDecodeSOARnameEmail(t *testing.T) {
	tests := []struct {
		name    string
		rname   string
		want    string
		wantOK  bool
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
