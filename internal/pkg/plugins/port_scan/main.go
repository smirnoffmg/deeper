package port_scan

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"

	"golang.org/x/sync/errgroup"
)

// defaultPorts is a small explicit set of commonly exposed TCP services. It is
// deliberately not the full 0-65535 range: a connect scan of every port would
// be slow and noisy, and this active plugin only aims to find the usual suspects.
var defaultPorts = []int{21, 22, 23, 25, 53, 80, 110, 143, 443, 445, 3306, 3389, 5432, 6379, 8080, 8443}

const bannerReadTimeout = time.Second

// dialer is the network boundary, injectable so tests never open real sockets.
type dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type PortScanPlugin struct {
	dialer      dialer
	ports       []int
	timeout     time.Duration
	concurrency int
}

func NewPlugin() *PortScanPlugin {
	return &PortScanPlugin{
		dialer:      &net.Dialer{},
		ports:       defaultPorts,
		timeout:     2 * time.Second,
		concurrency: 20,
	}
}

func (p *PortScanPlugin) Register(r plugins.Registry) error {
	r.Add(entities.IpAddr, p)
	r.Add(entities.Host, p)

	return nil
}

// Active marks this as active recon: it connects to the target directly, so the
// catalog enables it only under `scan --active`.
func (p *PortScanPlugin) Active() bool {
	return true
}

func (p *PortScanPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.IpAddr && trace.Type != entities.Host {
		return nil, nil
	}

	host := trace.Value

	limit := p.concurrency
	if limit < 1 {
		limit = 1
	}

	var (
		mu      sync.Mutex
		results []entities.Trace
		g       errgroup.Group
	)
	g.SetLimit(limit)

	for _, port := range p.ports {
		// A closed port is a normal result, not a group error, so the
		// closure never returns one -- errgroup here is bounded concurrency
		// (SetLimit) plus Wait, not first-error cancellation.
		if ctx.Err() != nil {
			break
		}
		g.Go(func() error {
			traces := p.scanPort(ctx, host, port)
			if len(traces) > 0 {
				mu.Lock()
				results = append(results, traces...)
				mu.Unlock()
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return dedupe(results), nil
}

func (p *PortScanPlugin) String() string {
	return "PortScanPlugin"
}

// scanPort attempts one TCP connect. A dial error (refused, timeout, cancelled)
// means the port is closed or unreachable -- never a plugin error.
func (p *PortScanPlugin) scanPort(ctx context.Context, host string, port int) []entities.Trace {
	address := net.JoinHostPort(host, strconv.Itoa(port))

	dialCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	conn, err := p.dialer.DialContext(dialCtx, "tcp", address)
	if err != nil {
		return nil
	}
	defer func() { _ = conn.Close() }()

	traces := []entities.Trace{{Value: address, Type: entities.Port}}

	if banner := readBanner(conn); banner != "" {
		traces = append(traces, entities.Trace{Value: banner, Type: entities.Service})
	}

	return traces
}

func readBanner(conn net.Conn) string {
	_ = conn.SetReadDeadline(time.Now().Add(bannerReadTimeout))

	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	if n == 0 {
		return ""
	}

	banner := strings.TrimSpace(string(buf[:n]))
	if banner == "" || !isPrintable(banner) {
		return ""
	}

	return banner
}

func isPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) && !unicode.IsSpace(r) {
			return false
		}
	}

	return true
}

func dedupe(traces []entities.Trace) []entities.Trace {
	if len(traces) == 0 {
		return nil
	}

	seen := make(map[entities.Trace]struct{}, len(traces))
	out := make([]entities.Trace, 0, len(traces))

	for _, t := range traces {
		if _, ok := seen[t]; ok {
			continue
		}

		seen[t] = struct{}{}
		out = append(out, t)
	}

	return out
}
