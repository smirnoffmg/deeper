package entities

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
		{"domain with hyphen in label", "registry-one.example.com", Domain},
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

func TestGuessTraceType_Deterministic(t *testing.T) {
	const repeats = 1000

	values := []string{
		"test@example.com",
		"+1-555-123-4567",
		"192.168.1.1",
		"example.com",
		"registry-one.example.com",
		"ab--cd.example.com",
		"xn--80ak6aa92e.com",
		"example.com.",
		"Registry-One.Example.COM",
		"https://example.com",
		"my-blog.tumblr.com",
		"blog.tumblr.com",
		"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		"randomusername",
		"-bad.example.com",
		"localhost",
	}

	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			want := guessTraceType(value)
			for i := range repeats {
				if got := guessTraceType(value); got != want {
					t.Fatalf("guessTraceType(%q) on repeat %d = %s, want %s (first result)", value, i, got, want)
				}
			}
		})
	}
}

func TestNewTrace_DomainCanonicalization(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{"trailing dot and mixed case", "Example.COM.", "example.com"},
		{"trailing dot, hyphenated label", "registry-one.example.com.", "registry-one.example.com"},
		{"mixed case, no trailing dot", "Registry-One.Example.COM", "registry-one.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := NewTrace(tt.value)
			if trace.Type != Domain {
				t.Fatalf("NewTrace(%q).Type = %s, want %s", tt.value, trace.Type, Domain)
			}
			if trace.Value != tt.expected {
				t.Errorf("NewTrace(%q).Value = %q, want %q", tt.value, trace.Value, tt.expected)
			}
		})
	}
}

// captureLog swaps the package-global log.Logger, so it is unsafe in any
// test that runs t.Parallel.
func captureLog(t *testing.T) *strings.Builder {
	t.Helper()
	var buf strings.Builder
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })
	return &buf
}

func TestGuessTraceType_LogsRollbackReason(t *testing.T) {
	buf := captureLog(t)

	const value = "bad-.example.com"
	if got := guessTraceType(value); got != Username {
		t.Fatalf("guessTraceType(%q) = %s, want %s", value, got, Username)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want exactly 1: %q", len(lines), buf.String())
	}

	var entry struct {
		Level  string `json:"level"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v (%q)", err, lines[0])
	}
	if entry.Level != "info" {
		t.Errorf("level = %q, want %q", entry.Level, "info")
	}
	if entry.Reason != "hyphen_edge" {
		t.Errorf("reason = %q, want %q", entry.Reason, "hyphen_edge")
	}
}

// Plain letters alone once hid a double-escaping bug: only escape-heavy bytes
// push the line past 1 KiB.
func TestGuessTraceType_LogsRollbackReason_TruncatesLongValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"plain letters", strings.Repeat("a", 64*1024)},
		{"NUL bytes", strings.Repeat("\x00", 64*1024)},
		{"invalid UTF-8 (0xff)", strings.Repeat("\xff", 64*1024)},
		{"quotes and backslashes", strings.Repeat(`"\`, 32*1024)},
		{"mixed control bytes", strings.Repeat("\x00\x01\x02\x1f\"\\", 64*1024/6)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureLog(t)

			if got := guessTraceType(tt.value); got != Username {
				t.Fatalf("guessTraceType(64 KiB value) = %s, want %s", got, Username)
			}

			line := strings.TrimSpace(buf.String())
			if n := len(line); n > 1024 {
				t.Fatalf("log line is %d bytes, want <= 1024", n)
			}

			var entry struct {
				ValueLen int `json:"value_len"`
			}
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				t.Fatalf("log line is not valid JSON: %v (%q)", err, line)
			}
			if entry.ValueLen != len(tt.value) {
				t.Errorf("value_len = %d, want %d", entry.ValueLen, len(tt.value))
			}
		})
	}
}

// Characterization table: pins the TraceType and, for domains, the canonical
// Value of every documented seed shape, including quirks kept on purpose.
func TestNewTrace_ReferenceTable(t *testing.T) {
	label64 := strings.Repeat("a", 64) + ".example.com"
	longLabel := strings.Repeat("a", 60)
	// 304 bytes of 60-byte labels: over the name limit, under the label limit.
	over253 := strings.Repeat(longLabel+".", 4) + longLabel

	addressAtCutoff := "123 Main St" + strings.Repeat("x", maxClassifiableLen-len("123 Main St"))
	addressOverCutoff := "123 Main St" + strings.Repeat("x", maxClassifiableLen+1-len("123 Main St"))
	emailOverCutoff := "a@" + strings.Repeat("a", maxClassifiableLen+1-len("a@")-len(".com")) + ".com"

	tests := []struct {
		name      string
		value     string
		wantType  TraceType
		wantValue string
		skip      string
	}{
		{name: "hyphen inside label", value: "registry-one.example.com", wantType: Domain, wantValue: "registry-one.example.com"},
		{name: "short hyphenated label", value: "a-b.example.com", wantType: Domain, wantValue: "a-b.example.com"},
		{name: "double hyphen not xn--", value: "ab--cd.example.com", wantType: Domain, wantValue: "ab--cd.example.com"},
		{name: "hyphenated numeric brand", value: "1-800-flowers.com", wantType: Domain, wantValue: "1-800-flowers.com"},
		{name: "hyphen in subdomain label", value: "my-name.github.io", wantType: Domain, wantValue: "my-name.github.io"},

		{name: "no-hyphen counterpart", value: "registryone.example.com", wantType: Domain, wantValue: "registryone.example.com"},

		// Invalid hostnames roll back to username; guarding that pipeline is issue #11.
		{name: "leading hyphen", value: "-bad.example.com", wantType: Username, wantValue: "-bad.example.com"},
		{name: "trailing hyphen", value: "bad-.example.com", wantType: Username, wantValue: "bad-.example.com"},
		{name: "empty label mid-string", value: "a..example.com", wantType: Username, wantValue: "a..example.com"},
		{name: "leading dot", value: ".example.com", wantType: Username, wantValue: ".example.com"},
		{name: "label over 63 bytes (regression from domain)", value: label64, wantType: Username, wantValue: label64},
		{name: "name over 253 bytes (regression from domain)", value: over253, wantType: Username, wantValue: over253},

		{name: "punycode label", value: "xn--80ak6aa92e.com", wantType: Domain, wantValue: "xn--80ak6aa92e.com"},
		{name: "punycode in last label", value: "example.xn--p1ai", wantType: Domain, wantValue: "example.xn--p1ai"},
		{name: "xn-- checked syntactically only, no decode", value: "xn--zz.com", wantType: Domain, wantValue: "xn--zz.com"},

		{name: "cyrillic TLD", value: "пример.рф", skip: "Unicode hosts: separate issue"},
		{name: "umlaut", value: "münchen.de", skip: "Unicode hosts: separate issue"},
		{name: "mixed script (cyrillic е)", value: "еxample.com", skip: "Unicode hosts: separate issue"},

		{name: "trailing dot", value: "example.com.", wantType: Domain, wantValue: "example.com"},
		{name: "trailing dot, hyphenated label", value: "registry-one.example.com.", wantType: Domain, wantValue: "registry-one.example.com"},
		{name: "double trailing dot", value: "example.com..", wantType: Username, wantValue: "example.com.."},

		{name: "mixed case with hyphen", value: "Registry-One.Example.COM", wantType: Domain, wantValue: "registry-one.example.com"},

		{name: "plain domain", value: "example.com", wantType: Domain, wantValue: "example.com"},
		{name: "subdomain", value: "sub.example.com", wantType: Domain, wantValue: "sub.example.com"},
		{name: "numeric label", value: "123.example.com", wantType: Domain, wantValue: "123.example.com"},
		{name: "three subdomains", value: "a.b.c.example.com", wantType: Domain, wantValue: "a.b.c.example.com"},
		{name: "upper case, no hyphen", value: "Example.COM", wantType: Domain, wantValue: "example.com"},

		{name: "co.uk itself", value: "co.uk", wantType: Domain, wantValue: "co.uk"},
		{name: "github.io itself", value: "github.io", wantType: Domain, wantValue: "github.io"},

		// No PSL check: dotted nicknames and private TLDs stay domain.
		{name: "nickname with a dot", value: "john.doe", wantType: Domain, wantValue: "john.doe"},
		{name: ".local", value: "server.local", wantType: Domain, wantValue: "server.local"},
		{name: ".internal", value: "host.internal", wantType: Domain, wantValue: "host.internal"},
		{name: ".onion", value: "abc.onion", wantType: Domain, wantValue: "abc.onion"},

		{name: "localhost", value: "localhost", wantType: Username, wantValue: "localhost"},
		{name: "bare nickname", value: "randomusername", wantType: Username, wantValue: "randomusername"},
		{name: "nickname with hyphen", value: "john-doe", wantType: Username, wantValue: "john-doe"},
		{name: "nickname with underscore", value: "john_doe", wantType: Username, wantValue: "john_doe"},
		{name: "nickname, hyphen both edges", value: "-john-", wantType: Username, wantValue: "-john-"},

		{name: "leading underscore label", value: "_dmarc.example.com", wantType: Username, wantValue: "_dmarc.example.com"},
		{name: "underscore mid-label", value: "my_host.example.com", wantType: Username, wantValue: "my_host.example.com"},

		{name: "wildcard", value: "*.example.com", wantType: Username, wantValue: "*.example.com"},
		{name: "path", value: "example.com/path", wantType: Username, wantValue: "example.com/path"},
		{name: "port", value: "example.com:8080", wantType: Username, wantValue: "example.com:8080"},

		{name: "url with hyphenated host", value: "https://registry-one.example.com", wantType: Url, wantValue: "https://registry-one.example.com"},
		{name: "url with trailing slash, unchanged", value: "https://registry-one.example.com/", wantType: Username, wantValue: "https://registry-one.example.com/"},
		{name: "email with hyphenated host", value: "test@registry-one.example.com", wantType: Email, wantValue: "test@registry-one.example.com"},
		{name: "phone, hyphen-separated", value: "555-123-4567", wantType: Phone, wantValue: "555-123-4567"},
		{name: "phone, dot-separated", value: "555.123.4567", wantType: Phone, wantValue: "555.123.4567"},
		{name: "ip address", value: "192.168.1.1", wantType: IpAddr, wantValue: "192.168.1.1"},

		{name: "invalid IPv4 octets, unchanged", value: "999.999.999.999", wantType: IpAddr, wantValue: "999.999.999.999"},
		{name: "IPv6 literal, unchanged", value: "2001:db8::1", wantType: Username, wantValue: "2001:db8::1"},
		{name: "three numeric labels, not a valid TLD", value: "1.2.3", wantType: Username, wantValue: "1.2.3"},

		// The domain matcher runs before tumblr's.
		{name: "hyphenated tumblr host", value: "my-blog.tumblr.com", wantType: Domain, wantValue: "my-blog.tumblr.com"},
		{name: "plain tumblr host", value: "blog.tumblr.com", wantType: Domain, wantValue: "blog.tumblr.com"},

		{name: "address at cutoff (1024 bytes)", value: addressAtCutoff, wantType: Address, wantValue: addressAtCutoff},
		{name: "address over cutoff (1025 bytes)", value: addressOverCutoff, wantType: Username, wantValue: addressOverCutoff},
		{name: "synthetic email over cutoff", value: emailOverCutoff, wantType: Username, wantValue: emailOverCutoff},
		{name: "64 KiB arbitrary bytes", value: hostname64KiB, wantType: Username, wantValue: hostname64KiB},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			trace := NewTrace(tt.value)
			if trace.Type != tt.wantType {
				t.Errorf("NewTrace(%q).Type = %s, want %s", tt.value, trace.Type, tt.wantType)
			}
			if trace.Value != tt.wantValue {
				t.Errorf("NewTrace(%q).Value = %q, want %q", tt.value, trace.Value, tt.wantValue)
			}
		})
	}
}

// Known quirk, kept on purpose: tumblrRegex accepts a leading hyphen that
// hostnameError rejects. Social-host matching was out of scope for issue #9.
func TestIsTumblrBlog_InvalidHyphenStillMatches(t *testing.T) {
	const value = "-bad.tumblr.com"
	got := NewTrace(value)
	if got.Type != Tumblr {
		t.Fatalf("NewTrace(%q).Type = %s, want %s (known quirk, intentionally unfixed)", value, got.Type, Tumblr)
	}
}

// Guards sit at each unbounded matcher's real protocol maximum and must be
// unaffected; the over-cutoff fixtures are the accepted loss of accuracy.
func TestGuessTraceType_LengthCutoff(t *testing.T) {
	padAddress := func(n int) string {
		const base = "123 Main St"
		return base + strings.Repeat("x", n-len(base))
	}
	syntheticEmail := func(n int) string {
		// local "a" + "@" + domain, domain = filler + ".com".
		domainLen := n - len("a@")
		return "a@" + strings.Repeat("a", domainLen-len(".com")) + ".com"
	}
	syntheticURL := func(n int) string {
		const scheme = "https://"
		hostLen := n - len(scheme)
		return scheme + strings.Repeat("a", hostLen-len(".com")) + ".com"
	}
	syntheticTumblr := func(n int) string {
		const suffix = ".tumblr.com"
		labelLen := n - len(suffix)
		// The trailing hyphen fails isDomain but not tumblrRegex; a valid
		// .tumblr.com host would classify as Domain, which runs first.
		return strings.Repeat("a", labelLen-1) + "-" + suffix
	}
	syntheticLinkedin := func(n int) string {
		const prefix, trailingSlash = "https://www.linkedin.com/in/", "/"
		slugLen := n - len(prefix) - len(trailingSlash)
		return prefix + strings.Repeat("a", slugLen) + trailingSlash
	}
	syntheticYouTube := func(n int) string {
		const prefix, trailingSlash = "https://www.youtube.com/channel/", "/"
		idLen := n - len(prefix) - len(trailingSlash)
		return prefix + strings.Repeat("a", idLen) + trailingSlash
	}

	guards := []struct {
		name     string
		value    string
		wantType TraceType
	}{
		{"address at cutoff (1024 bytes)", padAddress(maxClassifiableLen), Address},
		{"email at RFC 5321 path max (254 bytes)", syntheticEmail(254), Email},
		{"url at host max (261 bytes = 8 + 253)", syntheticURL(261), Url},
		{"tumblr matcher reachable only via invalid label (74 bytes = 63 + 11)", syntheticTumblr(74), Tumblr},
		{"valid *.tumblr.com host at label max is Domain (74 bytes = 63 + 11)", strings.Repeat("a", 63) + ".tumblr.com", Domain},
		{"linkedin at slug max (129 bytes)", syntheticLinkedin(129), Linkedin},
		{"youtube at channel ID max (57 bytes)", syntheticYouTube(57), YouTube},
	}
	for _, tt := range guards {
		t.Run(tt.name, func(t *testing.T) {
			if got := guessTraceType(tt.value); got != tt.wantType {
				t.Errorf("guessTraceType(%d bytes) = %s, want %s", len(tt.value), got, tt.wantType)
			}
		})
	}

	overCutoff := []struct {
		name  string
		value string
	}{
		{"address one byte over cutoff (1025 bytes)", padAddress(maxClassifiableLen + 1)},
		{"synthetic email over cutoff", syntheticEmail(1025)},
		{"synthetic url over cutoff", syntheticURL(1025)},
		{"synthetic tumblr over cutoff", syntheticTumblr(1025)},
		{"synthetic linkedin over cutoff", syntheticLinkedin(1025)},
		{"synthetic youtube over cutoff", syntheticYouTube(1025)},
	}
	for _, tt := range overCutoff {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureLog(t)

			if got := guessTraceType(tt.value); got != Username {
				t.Fatalf("guessTraceType(%d bytes) = %s, want %s", len(tt.value), got, Username)
			}

			line := strings.TrimSpace(buf.String())
			var entry struct {
				Reason   string `json:"reason"`
				ValueLen int    `json:"value_len"`
			}
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				t.Fatalf("log line is not valid JSON: %v (%q)", err, line)
			}
			if entry.Reason != "value_too_long" {
				t.Errorf("reason = %q, want %q", entry.Reason, "value_too_long")
			}
			if entry.ValueLen != len(tt.value) {
				t.Errorf("value_len = %d, want %d", entry.ValueLen, len(tt.value))
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
		{"hyphen inside label", "registry-one.example.com", true},
		{"short hyphenated label", "a-b.example.com", true},
		{"double hyphen not xn--", "ab--cd.example.com", true},
		{"hyphenated numeric brand", "1-800-flowers.com", true},
		{"hyphen in subdomain label", "my-name.github.io", true},
		{"leading hyphen invalid", "-bad.example.com", false},
		{"trailing hyphen invalid", "bad-.example.com", false},
		{"punycode label", "xn--80ak6aa92e.com", true},
		{"punycode in last label", "example.xn--p1ai", true},
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

func TestIsEmail_Exported(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{"valid email", "test@example.com", true},
		{"invalid, two at signs", "mailto@hello@codescoring.ru", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmail(tt.email)
			if result != tt.expected {
				t.Errorf("IsEmail(%s) = %t, want %t", tt.email, result, tt.expected)
			}
		})
	}
}

func TestIsFacebookProfile_Exported(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"valid profile", "https://www.facebook.com/someuser", true},
		{"bare host, no profile segment", "https://www.facebook.com/", false},
		{"sharer widget", "https://www.facebook.com/sharer/sharer.php", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsFacebookProfile(tt.url)
			if result != tt.expected {
				t.Errorf("IsFacebookProfile(%s) = %t, want %t", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsIpAddr_Exported(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"valid IPv4", "192.168.1.1", true},
		{"trailing CR", "192.168.1.1\r", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIpAddr(tt.ip)
			if result != tt.expected {
				t.Errorf("IsIpAddr(%s) = %t, want %t", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestIsRealEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{"real address", "hello@codescoring.ru", true},
		{"git unconfigured default", "you@example.com", false},
		{"reserved org domain", "test@example.org", false},
		{"reserved net domain", "test@example.net", false},
		{"case-insensitive reserved domain", "TEST@EXAMPLE.COM", false},
		{"malformed, not an email at all", "mailto@hello@codescoring.ru", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRealEmail(tt.email)
			if result != tt.expected {
				t.Errorf("IsRealEmail(%s) = %t, want %t", tt.email, result, tt.expected)
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
