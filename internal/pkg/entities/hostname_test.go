package entities

import (
	"strings"
	"testing"
)

func TestHostnameError(t *testing.T) {
	// Exactly 253 bytes: three 63-byte labels, one 61-byte label, three dots.
	maxLenValid := strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." +
		strings.Repeat("a", 63) + "." + strings.Repeat("a", 61)
	if len(maxLenValid) != 253 {
		t.Fatalf("test fixture maxLenValid has length %d, want 253", len(maxLenValid))
	}

	tests := []struct {
		name       string
		value      string
		wantReason string // "" means hostnameError must return nil
	}{
		{"empty string", "", "empty"},
		{"254 bytes, no dot", strings.Repeat("a", 254), "too_long"},
		{"exactly 253 bytes, valid", maxLenValid, ""},
		{"double trailing dot", "example.com..", "empty_label"},
		{"single label", "localhost", "single_label"},
		{"leading hyphen in label", "-bad.example.com", "hyphen_edge"},
		{"trailing hyphen in label", "bad-.example.com", "hyphen_edge"},
		{"empty label between dots", "a..example.com", "empty_label"},
		{"label over 63 bytes", strings.Repeat("a", 64) + ".example.com", "label_too_long"},
		{"hyphen inside label", "registry-one.example.com", ""},
		{"single-char labels around hyphen", "a-b.example.com", ""},
		{"double hyphen not at edges", "ab--cd.example.com", ""},
		{"hyphenated numeric brand", "1-800-flowers.com", ""},
		{"hyphen in subdomain label", "my-name.github.io", ""},
		{"punycode label lowercase", "xn--80ak6aa92e.com", ""},
		{"punycode tld", "example.xn--p1ai", ""},
		{"punycode uppercase mixed", "XN--zz.COM", ""},
		{"ipv4-like, non-alpha tld", "999.999.999.999", "invalid_tld"},
		{"three-label numeric", "1.2.3", "invalid_tld"},
		{"ipv6 literal, no dots", "2001:db8::1", "single_label"},
		{"mixed case preserved", "Registry-One.Example.COM", ""},
		{"underscore in label", "_dmarc.example.com", "invalid_char"},
		{"underscore mid-label", "my_host.example.com", "invalid_char"},
		{"asterisk not LDH", "a*b.example.com", "invalid_char"},
		{"colon not LDH", "a:b.example.com", "invalid_char"},
		{"slash not LDH", "a/b.example.com", "invalid_char"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hostnameError(tt.value)
			if tt.wantReason == "" {
				if err != nil {
					t.Errorf("hostnameError(%q) = %v, want nil", tt.value, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("hostnameError(%q) = nil, want reason %q", tt.value, tt.wantReason)
			}
			if err.Error() != tt.wantReason {
				t.Errorf("hostnameError(%q) = %q, want %q", tt.value, err.Error(), tt.wantReason)
			}
		})
	}
}

func FuzzHostnameError(f *testing.F) {
	seeds := []string{
		"",
		".",
		"..",
		"example.com",
		"example.com.",
		"registry-one.example.com",
		"xn--80ak6aa92e.com",
		"XN--zz.COM",
		string([]byte{0xff, 0xfe, 0xfd, 0x80}), // invalid UTF-8
		"\x00\x01\x02control.example.com",
		strings.Repeat("a", 65536),  // 64 KiB, no dots
		strings.Repeat(".", 65536),  // 64 KiB, all dots
		"😀.example.com",             // emoji, multi-byte rune
		"Registry-One.EXAMPLE.com.", // mixed case, trailing dot
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, value string) {
		err := hostnameError(value)
		if err == nil {
			if len(value) > maxHostnameLen+1 {
				t.Errorf("hostnameError(%q) = nil, but accepted value is %d bytes, want <= %d", value, len(value), maxHostnameLen+1)
			}
			for i := 0; i < len(value); i++ {
				b := value[i]
				if !isLDHByte(b) && b != '.' {
					t.Errorf("hostnameError(%q) = nil, but accepted value has non-LDH, non-dot byte %q at index %d", value, b, i)
				}
			}
		}
	})
}
