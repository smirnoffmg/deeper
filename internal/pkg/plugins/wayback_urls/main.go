package wayback_urls

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

const InputTraceType = entities.Domain

// maxURLs caps how many archived URLs a single domain contributes to the
// graph. A busy domain can have hundreds of thousands of archived URLs in the
// Wayback CDX index; emitting all of them would flood downstream plugins and
// the report, so we keep only the first maxURLs unique entries.
const maxURLs = 500

type urlFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

type WaybackURLsPlugin struct {
	fetcher urlFetcher
}

func NewPlugin(cfg *config.Config) *WaybackURLsPlugin {
	return &WaybackURLsPlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (p *WaybackURLsPlugin) Register(r plugins.Registry) error {
	r.Add(InputTraceType, p)
	return nil
}

func (p *WaybackURLsPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	url := fmt.Sprintf(
		"http://web.archive.org/cdx/search/cdx?url=*.%s/*&output=json&fl=original&collapse=urlkey",
		trace.Value,
	)

	resp, err := p.fetcher.Get(ctx, url)
	if err != nil {
		log.Warn().Err(err).Str("domain", trace.Value).Msg("wayback request failed, skipping")
		return nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.Warn().Int("status", resp.StatusCode).Str("domain", trace.Value).Msg("wayback returned non-200, skipping")
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warn().Err(err).Str("domain", trace.Value).Msg("wayback response read failed, skipping")
		return nil, nil
	}

	rows, ok := decodeRows(body, trace.Value)
	if !ok {
		return nil, nil
	}

	return rowsToTraces(rows), nil
}

// decodeRows parses the CDX JSON body, which is an array of arrays. The first
// row is a field header (["original"]) and carries no data.
func decodeRows(body []byte, domain string) ([][]string, bool) {
	if len(strings.TrimSpace(string(body))) == 0 {
		log.Warn().Str("domain", domain).Msg("wayback returned empty response, skipping")
		return nil, false
	}

	var rows [][]string
	if err := json.Unmarshal(body, &rows); err != nil {
		log.Warn().Err(err).Str("domain", domain).Msg("wayback returned invalid JSON, skipping")
		return nil, false
	}

	return rows, true
}

func rowsToTraces(rows [][]string) []entities.Trace {
	seen := make(map[string]struct{})
	var traces []entities.Trace

	for _, row := range rows[min(1, len(rows)):] {
		if len(row) == 0 {
			continue
		}
		u := strings.TrimSpace(row[0])
		if u == "" {
			continue
		}
		if _, dup := seen[u]; dup {
			continue
		}
		seen[u] = struct{}{}

		traces = append(traces, entities.Trace{Value: u, Type: entities.Url})
		if len(traces) >= maxURLs {
			break
		}
	}

	return traces
}

func (p WaybackURLsPlugin) String() string {
	return "WaybackURLsPlugin"
}
