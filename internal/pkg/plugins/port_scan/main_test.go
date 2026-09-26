package port_scan

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAddr is a stand-in net.Addr for fakeConn.
type fakeAddr struct{}

func (fakeAddr) Network() string { return "tcp" }
func (fakeAddr) String() string  { return "fake" }

// fakeConn is a minimal net.Conn whose Read is backed by a byte buffer, so a
// preloaded banner can be handed to the plugin without a real socket.
type fakeConn struct {
	r *bytes.Reader
}

func (c *fakeConn) Read(b []byte) (int, error)         { return c.r.Read(b) }
func (c *fakeConn) Write(b []byte) (int, error)        { return len(b), nil }
func (c *fakeConn) Close() error                       { return nil }
func (c *fakeConn) LocalAddr() net.Addr                { return fakeAddr{} }
func (c *fakeConn) RemoteAddr() net.Addr               { return fakeAddr{} }
func (c *fakeConn) SetDeadline(t time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(t time.Time) error { return nil }

// fakeDialer maps an "host:port" address to an optional banner. A present key
// means the port is open; an absent key means the dial is refused (closed).
type fakeDialer struct {
	open map[string]string
}

func (d *fakeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	banner, ok := d.open[address]
	if !ok {
		return nil, errors.New("connection refused")
	}

	return &fakeConn{r: bytes.NewReader([]byte(banner))}, nil
}

func newTestPlugin(dialer dialer, ports []int) *PortScanPlugin {
	return &PortScanPlugin{
		dialer:      dialer,
		ports:       ports,
		timeout:     time.Second,
		concurrency: 4,
	}
}

func TestFollowTrace_OnlyOpenPortsEmitted(t *testing.T) {
	d := &fakeDialer{open: map[string]string{
		"1.2.3.4:22":  "",
		"1.2.3.4:443": "",
	}}
	p := newTestPlugin(d, []int{22, 80, 443})

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Value: "1.2.3.4", Type: entities.IpAddr})
	require.NoError(t, err)

	assert.ElementsMatch(t, []entities.Trace{
		{Value: "1.2.3.4:22", Type: entities.Port},
		{Value: "1.2.3.4:443", Type: entities.Port},
	}, traces)
}

func TestFollowTrace_BannerEmittedAsService(t *testing.T) {
	d := &fakeDialer{open: map[string]string{
		"1.2.3.4:22": "SSH-2.0-OpenSSH_8.9\r\n",
	}}
	p := newTestPlugin(d, []int{22})

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Value: "1.2.3.4", Type: entities.IpAddr})
	require.NoError(t, err)

	assert.ElementsMatch(t, []entities.Trace{
		{Value: "1.2.3.4:22", Type: entities.Port},
		{Value: "SSH-2.0-OpenSSH_8.9", Type: entities.Service},
	}, traces)
}

func TestFollowTrace_NonPrintableBannerSkipped(t *testing.T) {
	d := &fakeDialer{open: map[string]string{
		"1.2.3.4:443": "\x00\x01\x02\x03",
	}}
	p := newTestPlugin(d, []int{443})

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Value: "1.2.3.4", Type: entities.IpAddr})
	require.NoError(t, err)

	assert.ElementsMatch(t, []entities.Trace{
		{Value: "1.2.3.4:443", Type: entities.Port},
	}, traces)
}

func TestFollowTrace_HostTraceScanned(t *testing.T) {
	d := &fakeDialer{open: map[string]string{
		"example.com:80": "",
	}}
	p := newTestPlugin(d, []int{80, 443})

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Value: "example.com", Type: entities.Host})
	require.NoError(t, err)

	assert.ElementsMatch(t, []entities.Trace{
		{Value: "example.com:80", Type: entities.Port},
	}, traces)
}

func TestFollowTrace_ContextCancelledReturnsPromptly(t *testing.T) {
	d := &fakeDialer{open: map[string]string{
		"1.2.3.4:22": "banner",
	}}
	p := newTestPlugin(d, []int{22, 80, 443, 8080})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	var traces []entities.Trace
	var err error
	go func() {
		traces, err = p.FollowTrace(ctx, entities.Trace{Value: "1.2.3.4", Type: entities.IpAddr})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("FollowTrace did not honour context cancellation promptly")
	}

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestFollowTrace_WrongTraceTypeReturnsNil(t *testing.T) {
	d := &fakeDialer{open: map[string]string{"1.2.3.4:22": ""}}
	p := newTestPlugin(d, []int{22})

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Value: "a@b.com", Type: entities.Email})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestActiveMarker(t *testing.T) {
	assert.True(t, NewPlugin().Active())
}

func TestString(t *testing.T) {
	assert.Equal(t, "PortScanPlugin", NewPlugin().String())
}
