package contact_crawler

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeIPResolver struct {
	ips map[string][]net.IP
	err error
}

func (f *fakeIPResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	if f.err != nil {
		return nil, f.err
	}
	ips, ok := f.ips[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host}
	}
	addrs := make([]net.IPAddr, len(ips))
	for i, ip := range ips {
		addrs[i] = net.IPAddr{IP: ip}
	}
	return addrs, nil
}

func TestRegistrableDomain(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
		ok   bool
	}{
		{name: "subdomain", host: "registry.codescoring.ru", want: "codescoring.ru", ok: true},
		{name: "www", host: "www.codescoring.ru", want: "codescoring.ru", ok: true},
		{name: "uppercase", host: "WWW.CODESCORING.RU", want: "codescoring.ru", ok: true},
		{name: "bare domain", host: "codescoring.ru", want: "codescoring.ru", ok: true},
		{name: "empty", host: "", want: "", ok: false},
		{name: "ip literal", host: "192.168.1.1", want: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := registrableDomain(tt.host)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSameSite(t *testing.T) {
	sharedIP := net.ParseIP("185.55.56.154")
	otherIP := net.ParseIP("10.0.0.1")

	resolver := &fakeIPResolver{ips: map[string][]net.IP{
		"codescoring.ru":          {sharedIP},
		"www.codescoring.ru":      {sharedIP},
		"registry.codescoring.ru": {sharedIP},
		"other.codescoring.ru":    {otherIP},
		"evil.com":                {otherIP},
		"codescoring.ru.evil.com": {otherIP},
		"evilcodescoring.ru":      {otherIP},
	}}
	checker := newSiteChecker(resolver)

	tests := []struct {
		name      string
		seed      string
		candidate string
		want      bool
	}{
		{name: "same IP via www", seed: "codescoring.ru", candidate: "www.codescoring.ru", want: true},
		{name: "same IP via subdomain", seed: "codescoring.ru", candidate: "registry.codescoring.ru", want: true},
		{name: "different IP same registrable", seed: "codescoring.ru", candidate: "other.codescoring.ru", want: false},
		{name: "unrelated domain", seed: "codescoring.ru", candidate: "evil.com", want: false},
		{name: "suffix trap evil subdomain", seed: "codescoring.ru", candidate: "codescoring.ru.evil.com", want: false},
		{name: "suffix trap evil prefix", seed: "codescoring.ru", candidate: "evilcodescoring.ru", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, checker.sameSite(tt.seed, tt.candidate))
		})
	}
}

func TestSameSite_IPLiteral(t *testing.T) {
	ip := net.ParseIP("185.55.56.154")
	checker := newSiteChecker(&fakeIPResolver{ips: map[string][]net.IP{
		"codescoring.ru": {ip},
	}})

	assert.True(t, checker.sameSite("185.55.56.154", "codescoring.ru"))
	assert.False(t, checker.sameSite("10.0.0.1", "codescoring.ru"))
}
