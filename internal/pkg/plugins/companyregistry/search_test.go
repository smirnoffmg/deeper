package companyregistry

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realFixture mirrors the actual markup captured from list-org.com's search
// results page for a query that matches ООО "ПРОФИСКОП" (INN 7813227385).
const realFixture = `<div class='card w-100 p-1 p-lg-3 mt-1'><div class='org_list'><p><label><input class='form-check-input' data-id='1412885' type='checkbox' checked><a href='/company/1412885'>ООО "ПРОФИСКОП"</a><br><span>ОБЩЕСТВО С ОГРАНИЧЕННОЙ ОТВЕТСТВЕННОСТЬЮ "ПРОФИСКОП"<br><i>руководитель</i>: СМИРНОВ АЛЕКСЕЙ АЛЕКСЕЕВИЧ<br><i>инн/кпп</i>: 7813227385/781001001<br><i>юр.адрес</i>: 196105, САНКТ-ПЕТЕРБУРГ ГОРОД, УЛИЦА СВЕАБОРГСКАЯ, Д. 4, ПОМЕЩ. 9</span></label></p>`

func TestSearchCompany_ExtractsCompanyDirectorAndAddress(t *testing.T) {
	fetcher := &fakeSearchFetcher{
		responses: map[string]fakeResponse{
			expectedListOrgURL(t, "7813227385"): {status: http.StatusOK, body: realFixture},
		},
	}

	traces, err := searchCompany(context.Background(), fetcher, "7813227385")
	require.NoError(t, err)

	want := map[entities.TraceType]string{
		entities.Company: `ОБЩЕСТВО С ОГРАНИЧЕННОЙ ОТВЕТСТВЕННОСТЬЮ "ПРОФИСКОП"`,
		entities.Name:    "СМИРНОВ АЛЕКСЕЙ АЛЕКСЕЕВИЧ",
		entities.Address: "196105, САНКТ-ПЕТЕРБУРГ ГОРОД, УЛИЦА СВЕАБОРГСКАЯ, Д. 4, ПОМЕЩ. 9",
	}
	got := map[entities.TraceType]string{}
	for _, tr := range traces {
		got[tr.Type] = tr.Value
	}
	for traceType, wantValue := range want {
		assert.Equal(t, wantValue, got[traceType], "type %v", traceType)
	}
}

func TestSearchCompany_NoMatchReturnsNoTraces(t *testing.T) {
	fetcher := &fakeSearchFetcher{
		responses: map[string]fakeResponse{
			expectedListOrgURL(t, "nothing"): {status: http.StatusOK, body: `<h1>Список организаций "nothing" Найдено </h1>`},
		},
	}

	traces, err := searchCompany(context.Background(), fetcher, "nothing")
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestSearchCompany_NonOKStatusReturnsError(t *testing.T) {
	fetcher := &fakeSearchFetcher{
		responses: map[string]fakeResponse{
			expectedListOrgURL(t, "7813227385"): {status: http.StatusForbidden, body: ""},
		},
	}

	_, err := searchCompany(context.Background(), fetcher, "7813227385")
	assert.Error(t, err)
}

func expectedListOrgURL(t *testing.T, query string) string {
	t.Helper()
	return "https://www.list-org.com/search?val=" + url.QueryEscape(query)
}

type fakeResponse struct {
	status int
	body   string
}

type fakeSearchFetcher struct {
	responses map[string]fakeResponse
	lastURL   string
}

func (f *fakeSearchFetcher) Get(_ context.Context, requestURL string) (*http.Response, error) {
	f.lastURL = requestURL

	resp, ok := f.responses[requestURL]
	if !ok {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	return &http.Response{StatusCode: resp.status, Body: io.NopCloser(strings.NewReader(resp.body))}, nil
}
