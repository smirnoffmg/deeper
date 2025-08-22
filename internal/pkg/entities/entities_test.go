package entities

import (
	"testing"
)

func TestNewTrace(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected TraceType
	}{
		{"email", "test@example.com", Email},
		{"phone", "+1-555-123-4567", Phone},
		{"ip_addr", "192.168.1.1", IpAddr},
		{"domain", "example.com", Domain},
		{"url", "https://example.com", Url},
		{"address", "123 Main St", Address},
		{"twitter", "@username", Twitter},
		{"linkedin", "https://linkedin.com/in/username", Linkedin},
		{"facebook", "https://facebook.com/username", Facebook},
		{"reddit", "u/username", Reddit},
		{"youtube", "https://youtube.com/channel/username", YouTube},
		{"pinterest", "https://pinterest.com/username", Pinterest},
		{"mac_addr", "00:11:22:33:44:55", MacAddr},
		{"bitcoin", "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", BitcoinAddress},
		{"username", "randomusername", Username},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := NewTrace(tt.value)
			if trace.Type != tt.expected {
				t.Errorf("NewTrace(%s) = %s, want %s", tt.value, trace.Type, tt.expected)
			}
			if trace.Value != tt.value {
				t.Errorf("NewTrace(%s).Value = %s, want %s", tt.value, trace.Value, tt.value)
			}
		})
	}
}

func TestTraceString(t *testing.T) {
	trace := Trace{Value: "test@example.com", Type: Email}
	expected := "test@example.com (email)"
	if trace.String() != expected {
		t.Errorf("Trace.String() = %s, want %s", trace.String(), expected)
	}
}

func TestIsEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{"valid email", "test@example.com", true},
		{"valid email with subdomain", "test@sub.example.com", true},
		{"valid email with plus", "test+tag@example.com", true},
		{"valid email with dots", "test.name@example.com", true},
		{"invalid email no domain", "test@", false},
		{"invalid email no at", "testexample.com", false},
		{"invalid email no local", "@example.com", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isEmail(tt.email)
			if result != tt.expected {
				t.Errorf("isEmail(%s) = %t, want %t", tt.email, result, tt.expected)
			}
		})
	}
}

func TestIsPhone(t *testing.T) {
	tests := []struct {
		name     string
		phone    string
		expected bool
	}{
		{"valid US phone", "+1-555-123-4567", true},
		{"valid phone with spaces", "555 123 4567", true},
		{"valid phone with dots", "555.123.4567", true},
		{"valid phone with parentheses", "(555) 123-4567", true},
		{"valid phone without country code", "555-123-4567", true},
		{"invalid phone too short", "123-456", false},
		{"invalid phone letters", "abc-def-ghij", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPhone(tt.phone)
			if result != tt.expected {
				t.Errorf("isPhone(%s) = %t, want %t", tt.phone, result, tt.expected)
			}
		})
	}
}

func TestIsIpAddr(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"valid IPv4", "192.168.1.1", true},
		{"valid IPv4", "10.0.0.1", true},
		{"valid IPv4", "172.16.0.1", true},
		{"invalid IP format", "192.168.1", false},
		{"invalid IP format", "192.168.1", false},
		{"invalid IP with letters", "192.168.1.a", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isIpAddr(tt.ip)
			if result != tt.expected {
				t.Errorf("isIpAddr(%s) = %t, want %t", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestIsDomain(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		expected bool
	}{
		{"valid domain", "example.com", true},
		{"valid subdomain", "sub.example.com", true},
		{"valid domain with multiple subdomains", "a.b.c.example.com", true},
		{"invalid domain no TLD", "example", false},
		{"invalid domain with protocol", "https://example.com", false},
		{"invalid domain with path", "example.com/path", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDomain(tt.domain)
			if result != tt.expected {
				t.Errorf("isDomain(%s) = %t, want %t", tt.domain, result, tt.expected)
			}
		})
	}
}

func TestIsUrl(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"valid HTTP URL", "http://example.com", true},
		{"valid HTTPS URL", "https://example.com", true},
		{"valid URL with subdomain", "https://sub.example.com", true},
		{"invalid URL no protocol", "example.com", false},
		{"invalid URL wrong protocol", "ftp://example.com", false},
		{"invalid URL no domain", "https://", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUrl(tt.url)
			if result != tt.expected {
				t.Errorf("isUrl(%s) = %t, want %t", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsBitcoinAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected bool
	}{
		{"valid Bitcoin address", "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", true},
		{"invalid address too short", "1A1zP1eP5QGefi2DMPTfTL5SL", false},
		{"invalid address wrong prefix", "2A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", false},
		{"invalid address with invalid chars", "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfN!", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBitcoinAddress(tt.address)
			if result != tt.expected {
				t.Errorf("isBitcoinAddress(%s) = %t, want %t", tt.address, result, tt.expected)
			}
		})
	}
}
