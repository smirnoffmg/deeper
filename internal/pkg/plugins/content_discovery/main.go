package content_discovery

import (
	"context"
	"net/http"
	"net/url"
	"sync"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"

	"golang.org/x/sync/errgroup"
)

// defaultWords is a deliberately tiny list of high-signal paths worth probing
// on almost any host. It is not a full brute-force dictionary: content
// discovery here is a quick sanity sweep, not an exhaustive scan.
var defaultWords = []string{
	"robots.txt",
	"sitemap.xml",
	".well-known/security.txt",
	".env",
	".git/config",
	"admin",
	"login",
	"wp-login.php",
	"api",
	"backup.zip",
}

// foundStatuses marks HTTP statuses that mean a path exists -- including 401/403
// (present but access-controlled) and redirects, which still reveal the path.
var foundStatuses = map[int]bool{
	http.StatusOK:                true,
	http.StatusNoContent:         true,
	http.StatusMovedPermanently:  true,
	http.StatusFound:             true,
	http.StatusTemporaryRedirect: true,
	http.StatusUnauthorized:      true,
	http.StatusForbidden:         true,
}

type pathFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

type ContentDiscoveryPlugin struct {
	fetcher     pathFetcher
	words       []string
	concurrency int
}

func NewPlugin(cfg *config.Config) *ContentDiscoveryPlugin {
	return &ContentDiscoveryPlugin{fetcher: deeperhttp.NewClient(cfg), words: defaultWords, concurrency: 10}
}

func (p *ContentDiscoveryPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Url, p)
	return nil
}

// Active marks this as an active-recon plugin: it sends many requests directly
// to the scan target, so the catalog enables it only under scan --active.
func (p *ContentDiscoveryPlugin) Active() bool {
	return true
}

func (p *ContentDiscoveryPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Url {
		return nil, nil
	}

	origin, ok := originOf(trace.Value)
	if !ok {
		return nil, nil
	}

	limit := p.concurrency
	if limit < 1 {
		limit = 1
	}

	var (
		mu   sync.Mutex
		seen = make(map[string]struct{})
		out  []entities.Trace
		g    errgroup.Group
	)
	g.SetLimit(limit)

	for _, word := range p.words {
		// A missing path is a normal result, not a group error, so the
		// closure returns nil; errgroup provides bounded concurrency + Wait.
		if ctx.Err() != nil {
			break
		}
		g.Go(func() error {
			fullURL := origin + "/" + word
			if !p.exists(ctx, fullURL) {
				return nil
			}
			mu.Lock()
			if _, dup := seen[fullURL]; !dup {
				seen[fullURL] = struct{}{}
				out = append(out, entities.Trace{Value: fullURL, Type: entities.Url})
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

func (p *ContentDiscoveryPlugin) exists(ctx context.Context, fullURL string) bool {
	resp, err := p.fetcher.Get(ctx, fullURL)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return foundStatuses[resp.StatusCode]
}

func (p *ContentDiscoveryPlugin) String() string {
	return "ContentDiscoveryPlugin"
}

func originOf(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}
