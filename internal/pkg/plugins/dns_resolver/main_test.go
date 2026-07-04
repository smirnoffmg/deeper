package dns_resolver

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResolver struct {
	addrs []net.IPAddr
	err   error
}

func (f *fakeResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f.addrs, f.err
}

func TestDNSResolverPlugin_FollowTrace_WrongType(t *testing.T) {
	plugin := &DNSResolverPlugin{resolver: &fakeResolver{}}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "codescoring.ru", Type: entities.Domain})

	require.NoError(t, err)
	assert.Empty(t, traces)
}

func TestDNSResolverPlugin_FollowTrace_ResolvesAddresses(t *testing.T) {
	plugin := &DNSResolverPlugin{resolver: &fakeResolver{
		addrs: []net.IPAddr{
			{IP: net.ParseIP("185.55.56.154")},
			{IP: net.ParseIP("2001:db8::1")},
		},
	}}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "registry.codescoring.ru", Type: entities.Subdomain})

	require.NoError(t, err)
	require.Len(t, traces, 2)
	assert.Equal(t, entities.IpAddr, traces[0].Type)
	assert.Equal(t, "185.55.56.154", traces[0].Value)
	assert.Equal(t, "2001:db8::1", traces[1].Value)
}

func TestDNSResolverPlugin_FollowTrace_LookupError(t *testing.T) {
	plugin := &DNSResolverPlugin{resolver: &fakeResolver{err: errors.New("no such host")}}

	traces, err := plugin.FollowTrace(entities.Trace{Value: "nonexistent.codescoring.ru", Type: entities.Subdomain})

	require.Error(t, err)
	assert.Empty(t, traces)
}

func TestDNSResolverPlugin_String(t *testing.T) {
	plugin := NewPlugin()
	assert.Equal(t, "DNSResolverPlugin", plugin.String())
}
