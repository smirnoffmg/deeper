package exif_meta

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type imageFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

type ExifMetaPlugin struct {
	fetcher imageFetcher
}

func NewPlugin(cfg *config.Config) *ExifMetaPlugin {
	return &ExifMetaPlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (p *ExifMetaPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Url, p)
	return nil
}

// goexif reads EXIF only from JPEG and TIFF containers; PNG carries none.
var imageExtensions = []string{".jpg", ".jpeg", ".tif", ".tiff"}

func isImageURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	path := strings.ToLower(u.Path)
	for _, ext := range imageExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}

	return false
}

func (p *ExifMetaPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Url || !isImageURL(trace.Value) {
		return nil, nil
	}

	resp, err := p.fetcher.Get(ctx, trace.Value)
	if err != nil {
		log.Warn().Err(err).Str("url", trace.Value).Msg("exif_meta: image fetch failed, skipping")
		return nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.Warn().Int("status", resp.StatusCode).Str("url", trace.Value).Msg("exif_meta: unexpected status, skipping")
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warn().Err(err).Str("url", trace.Value).Msg("exif_meta: reading image body failed, skipping")
		return nil, nil
	}

	return extractExifTraces(body), nil
}

func extractExifTraces(body []byte) []entities.Trace {
	x, err := exif.Decode(bytes.NewReader(body))
	if err != nil {
		log.Warn().Err(err).Msg("exif_meta: no decodable EXIF metadata, skipping")
		return nil
	}

	var traces []entities.Trace
	seen := make(map[entities.Trace]struct{})
	add := func(value string, traceType entities.TraceType) {
		if value == "" {
			return
		}

		tr := entities.Trace{Value: value, Type: traceType}
		if _, ok := seen[tr]; ok {
			return
		}

		seen[tr] = struct{}{}
		traces = append(traces, tr)
	}

	if lat, long, err := x.LatLong(); err == nil {
		add(fmt.Sprintf("%f,%f", lat, long), entities.Geolocation)
	}

	if tag, err := x.Get(exif.Artist); err == nil {
		if artist, err := tag.StringVal(); err == nil {
			add(cleanTagValue(artist), entities.Name)
		}
	}

	if tag, err := x.Get(exif.DateTimeOriginal); err == nil {
		if ts, err := tag.StringVal(); err == nil {
			add(cleanTagValue(ts), entities.FileTimestamp)
		}
	}

	return traces
}

func cleanTagValue(value string) string {
	value = strings.Trim(value, "\x00")
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	return strings.TrimSpace(value)
}

func (p *ExifMetaPlugin) String() string {
	return "ExifMetaPlugin"
}
