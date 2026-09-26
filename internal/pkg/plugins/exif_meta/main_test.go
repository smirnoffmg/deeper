package exif_meta

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jpegWithExif is a 2x2 JPEG carrying EXIF: GPS 37.774900,-122.419400,
// Artist "Jane Photographer", DateTimeOriginal "2021:07:14 09:30:00".
// Generated with Pillow; see the plugin's report for how it was produced.
const jpegWithExif = "/9j/4AAQSkZJRgABAQAAAQABAAD/4QDYRXhpZgAATU0AKgAAAAgAAwE7AAIAAAASAAAAModpAAQAAAABAAAARIglAAQAAAABAAAAagAAAABKYW5lIFBob3RvZ3JhcGhlcgAAAZADAAIAAAAUAAAAVgAAAAAyMDIxOjA3OjE0IDA5OjMwOjAwAAAEAAEAAgAAAAJOAAAAAAIABQAAAAMAAACgAAMAAgAAAAJXAAAAAAQABQAAAAMAAAC4AAAAAAAAACUAAAABAAAALgAAAAEAAALlAAAAGQAAAHoAAAABAAAAGQAAAAEAAAD2AAAAGf/bAEMACAYGBwYFCAcHBwkJCAoMFA0MCwsMGRITDxQdGh8eHRocHCAkLicgIiwjHBwoNyksMDE0NDQfJzk9ODI8LjM0Mv/bAEMBCQkJDAsMGA0NGDIhHCEyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMv/AABEIAAIAAgMBIgACEQEDEQH/xAAfAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgv/xAC1EAACAQMDAgQDBQUEBAAAAX0BAgMABBEFEiExQQYTUWEHInEUMoGRoQgjQrHBFVLR8CQzYnKCCQoWFxgZGiUmJygpKjQ1Njc4OTpDREVGR0hJSlNUVVZXWFlaY2RlZmdoaWpzdHV2d3h5eoOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4eLj5OXm5+jp6vHy8/T19vf4+fr/xAAfAQADAQEBAQEBAQEBAAAAAAAAAQIDBAUGBwgJCgv/xAC1EQACAQIEBAMEBwUEBAABAncAAQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXxFxgZGiYnKCkqNTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2gAMAwEAAhEDEQA/AOfooor7Q+IP/9k="

type fakeResponse struct {
	status int
	body   []byte
	err    error
}

type fakeImageFetcher struct {
	responses map[string]fakeResponse
	calls     int
	lastURL   string
}

func (f *fakeImageFetcher) Get(_ context.Context, url string) (*http.Response, error) {
	f.calls++
	f.lastURL = url

	resp, ok := f.responses[url]
	if !ok {
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	if resp.err != nil {
		return nil, resp.err
	}

	return &http.Response{StatusCode: resp.status, Body: io.NopCloser(strings.NewReader(string(resp.body)))}, nil
}

func decodeFixture(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(jpegWithExif)
	require.NoError(t, err)
	return data
}

func TestFollowTrace_ExtractsExifTraces(t *testing.T) {
	const imgURL = "https://example.com/photos/vacation.jpg?token=abc"
	fetcher := &fakeImageFetcher{
		responses: map[string]fakeResponse{
			imgURL: {status: http.StatusOK, body: decodeFixture(t)},
		},
	}
	p := &ExifMetaPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: imgURL})
	require.NoError(t, err)

	byType := map[entities.TraceType]string{}
	for _, tr := range traces {
		byType[tr.Type] = tr.Value
	}

	assert.Equal(t, "37.774900,-122.419400", byType[entities.Geolocation])
	assert.Equal(t, "Jane Photographer", byType[entities.Name])
	assert.Equal(t, "2021:07:14 09:30:00", byType[entities.FileTimestamp])
	assert.Len(t, traces, 3)
}

func TestFollowTrace_NonImageURL_NoFetch(t *testing.T) {
	fetcher := &fakeImageFetcher{}
	p := &ExifMetaPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: "https://example.com/page.html"})
	require.NoError(t, err)
	assert.Nil(t, traces)
	assert.Zero(t, fetcher.calls)
}

func TestFollowTrace_WrongTraceType(t *testing.T) {
	fetcher := &fakeImageFetcher{}
	p := &ExifMetaPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
	assert.Zero(t, fetcher.calls)
}

func TestFollowTrace_HTTPError(t *testing.T) {
	const imgURL = "https://example.com/broken.jpg"
	fetcher := &fakeImageFetcher{
		responses: map[string]fakeResponse{
			imgURL: {err: io.ErrUnexpectedEOF},
		},
	}
	p := &ExifMetaPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: imgURL})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_BadStatus(t *testing.T) {
	const imgURL = "https://example.com/missing.jpg"
	fetcher := &fakeImageFetcher{
		responses: map[string]fakeResponse{
			imgURL: {status: http.StatusInternalServerError, body: []byte("boom")},
		},
	}
	p := &ExifMetaPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: imgURL})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_BodyWithoutExif(t *testing.T) {
	const imgURL = "https://example.com/plain.jpg"
	fetcher := &fakeImageFetcher{
		responses: map[string]fakeResponse{
			imgURL: {status: http.StatusOK, body: []byte("this is not a valid exif image")},
		},
	}
	p := &ExifMetaPlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Url, Value: imgURL})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestRegister_RegistersUnderUrl(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	found := false
	for _, registered := range registry[entities.Url] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestString(t *testing.T) {
	assert.Equal(t, "ExifMetaPlugin", (&ExifMetaPlugin{}).String())
}
