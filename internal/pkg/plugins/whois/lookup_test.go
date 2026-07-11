package whois

import (
	"context"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupWhois_TwoHopParsesKeyValueLines(t *testing.T) {
	client := &fakeWhoisClient{responses: map[string]string{
		key(ianaWhoisServer, "ru"):        "whois: 127.0.0.1\n",
		key("127.0.0.1:43", "example.ru"): "% comment, skipped\n\ndomain: example.ru\norg: Example Corp\n",
	}}

	traces, err := lookupWhois(context.Background(), client, "example.ru")
	require.NoError(t, err)
	require.Len(t, traces, 2)
	assert.Equal(t, entities.Whois, traces[0].Type)
	assert.Equal(t, "domain: example.ru", traces[0].Value)
	assert.Equal(t, "org: Example Corp", traces[1].Value)
}

func TestLookupWhois_TaxpayerIDAlsoEmitsCompanyTrace(t *testing.T) {
	client := &fakeWhoisClient{responses: map[string]string{
		key(ianaWhoisServer, "ru"):        "whois: 127.0.0.1\n",
		key("127.0.0.1:43", "example.ru"): "domain: example.ru\norg: Example Corp\ntaxpayer-id: 1234567890\n",
	}}

	traces, err := lookupWhois(context.Background(), client, "example.ru")
	require.NoError(t, err)

	var companyTraces []entities.Trace
	for _, tr := range traces {
		if tr.Type == entities.Company {
			companyTraces = append(companyTraces, tr)
		}
	}
	require.Len(t, companyTraces, 1)
	assert.Equal(t, "1234567890", companyTraces[0].Value)

	for _, tr := range traces {
		assert.Falsef(t, tr.Type == entities.Company && tr.Value == "Example Corp",
			"org: must not produce a Company trace directly, got %v", tr)
	}
}

func TestLookupWhois_NoReferralLineReturnsNoTraces(t *testing.T) {
	client := &fakeWhoisClient{responses: map[string]string{
		key(ianaWhoisServer, "ru"): "% no whois or refer field\n",
	}}

	traces, err := lookupWhois(context.Background(), client, "example.ru")
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestLookupWhois_IANAQueryErrorPropagates(t *testing.T) {
	client := &fakeWhoisClient{responses: map[string]string{}, errOnMiss: true}

	_, err := lookupWhois(context.Background(), client, "example.ru")
	assert.Error(t, err)
}

func key(address, term string) string {
	return address + "\x00" + term
}

type fakeWhoisClient struct {
	responses map[string]string
	errOnMiss bool
	called    bool
}

func (f *fakeWhoisClient) Query(_ context.Context, address, term string) (string, error) {
	f.called = true
	resp, ok := f.responses[key(address, term)]
	if !ok {
		if f.errOnMiss {
			return "", assertUnreachable()
		}
		return "", nil
	}
	return resp, nil
}

func (f *fakeWhoisClient) lastQueried() bool {
	return f.called
}

func assertUnreachable() error {
	return context.DeadlineExceeded
}
