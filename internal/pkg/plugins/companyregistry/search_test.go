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

// realDetailFixture mirrors the actual markup captured from list-org.com's
// company detail page (/company/1412885) for ООО "ПРОФИСКОП".
const realDetailFixture = `<i>Предприятия рядом:</i> <span class='upper'> <a href='/company/1692761'>ООО "ЛИДЕР"</a>,  <a href='/company/1407264'>ООО "АТМ78"</a>,  <a href='/company/1592570'>ООО "СЗТК ИДЕАЛ АВТО"</a>,  <a href='/company/1412931'>ООО  "СТРОЙ ПЛЮС"</a><!--/noindex--></span>`

func expectedCompanyDetailURL(id string) string {
	return "https://www.list-org.com/company/" + id
}

func TestSearchCompany_IncludesNearbyCompanies(t *testing.T) {
	fetcher := &fakeSearchFetcher{
		responses: map[string]fakeResponse{
			expectedListOrgURL(t, "7813227385"): {status: http.StatusOK, body: realFixture},
			expectedCompanyDetailURL("1412885"): {status: http.StatusOK, body: realDetailFixture},
		},
	}

	traces, err := searchCompany(context.Background(), fetcher, "7813227385")
	require.NoError(t, err)

	var nearby []string
	for _, tr := range traces {
		if tr.Type == entities.Company {
			nearby = append(nearby, tr.Value)
		}
	}
	assert.Contains(t, nearby, `ОБЩЕСТВО С ОГРАНИЧЕННОЙ ОТВЕТСТВЕННОСТЬЮ "ПРОФИСКОП"`)
	assert.Contains(t, nearby, `ООО "ЛИДЕР"`)
	assert.Contains(t, nearby, `ООО "АТМ78"`)
	assert.Contains(t, nearby, `ООО "СЗТК ИДЕАЛ АВТО"`)
	assert.Contains(t, nearby, `ООО  "СТРОЙ ПЛЮС"`)
}

func TestSearchCompany_DetailPageFailureStillReturnsSearchTraces(t *testing.T) {
	fetcher := &fakeSearchFetcher{
		responses: map[string]fakeResponse{
			expectedListOrgURL(t, "7813227385"): {status: http.StatusOK, body: realFixture},
			expectedCompanyDetailURL("1412885"): {status: http.StatusForbidden, body: ""},
		},
	}

	traces, err := searchCompany(context.Background(), fetcher, "7813227385")
	require.NoError(t, err)
	require.Len(t, traces, 3, "original search-page traces must survive a detail-page failure")
}

func TestParseNearbyCompanies(t *testing.T) {
	names := parseNearbyCompanies(realDetailFixture)
	assert.Equal(t, []string{`ООО "ЛИДЕР"`, `ООО "АТМ78"`, `ООО "СЗТК ИДЕАЛ АВТО"`, `ООО  "СТРОЙ ПЛЮС"`}, names)
}

func TestParseNearbyCompanies_NoSectionReturnsNil(t *testing.T) {
	assert.Nil(t, parseNearbyCompanies(`<div>nothing here</div>`))
}

func TestExtractCompanyID(t *testing.T) {
	id, ok := extractCompanyID(realFixture)
	require.True(t, ok)
	assert.Equal(t, "1412885", id)
}

func TestExtractCompanyID_NotFound(t *testing.T) {
	_, ok := extractCompanyID(`<div>no link here</div>`)
	assert.False(t, ok)
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
