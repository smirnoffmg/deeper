package contact_crawler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	tests := []struct {
		name      string
		seed      string
		candidate string
		want      bool
	}{
		{name: "identical host", seed: "codescoring.ru", candidate: "codescoring.ru", want: true},
		{name: "www subdomain", seed: "codescoring.ru", candidate: "www.codescoring.ru", want: true},
		{name: "arbitrary subdomain", seed: "codescoring.ru", candidate: "registry.codescoring.ru", want: true},
		{name: "seed is a subdomain", seed: "registry.codescoring.ru", candidate: "other.codescoring.ru", want: true},
		{name: "unrelated domain", seed: "codescoring.ru", candidate: "evil.com", want: false},
		{name: "suffix trap evil subdomain", seed: "codescoring.ru", candidate: "codescoring.ru.evil.com", want: false},
		{name: "suffix trap evil prefix", seed: "codescoring.ru", candidate: "evilcodescoring.ru", want: false},
		{name: "ip literal candidate", seed: "codescoring.ru", candidate: "185.55.56.154", want: false},
		{name: "ip literal seed", seed: "185.55.56.154", candidate: "codescoring.ru", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sameSite(tt.seed, tt.candidate))
		})
	}
}
