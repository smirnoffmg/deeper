package entities

import (
	"errors"
	"strings"
)

// Error() text doubles as the rollback log's reason code.
var (
	errHostnameEmpty        = errors.New("empty")
	errHostnameTooLong      = errors.New("too_long")
	errHostnameSingleLabel  = errors.New("single_label")
	errHostnameEmptyLabel   = errors.New("empty_label")
	errHostnameLabelTooLong = errors.New("label_too_long")
	errHostnameInvalidChar  = errors.New("invalid_char")
	errHostnameHyphenEdge   = errors.New("hyphen_edge")
	errHostnameInvalidTLD   = errors.New("invalid_tld")

	// Not a hostnameError result: guessTraceType returns it before any
	// matcher, isDomain included, sees the value.
	errValueTooLong = errors.New("value_too_long")
)

const (
	maxHostnameLen = 253
	maxLabelLen    = 63

	// Several matchers have no length limit, so without this bound their
	// regex cost grows with untrusted input. It is ~3.9x the longest real
	// maximum (a URL host, 261 bytes), so only already-invalid values lose.
	maxClassifiableLen = 1024
)

// hostnameError reports why value is not a syntactically valid hostname per
// RFC 1123 §2.1 and RFC 3696 §2, or nil if it is valid. It works on raw bytes
// and consults no reference data (no PSL, no network), so it is deterministic
// and safe on non-UTF-8 input.
func hostnameError(value string) error {
	if value == "" {
		return errHostnameEmpty
	}

	// A second trailing dot surfaces below as an empty label.
	trimmed := strings.TrimSuffix(value, ".")
	if trimmed == "" {
		return errHostnameEmpty
	}

	if len(trimmed) > maxHostnameLen {
		return errHostnameTooLong
	}

	// Labels are walked in place rather than split: the success path must
	// not allocate (see BenchmarkHostnameError).
	if strings.Count(trimmed, ".")+1 < 2 {
		return errHostnameSingleLabel
	}

	lastLabelStart := 0
	start := 0
	for i := 0; i <= len(trimmed); i++ {
		if i < len(trimmed) && trimmed[i] != '.' {
			continue
		}
		label := trimmed[start:i]
		if len(label) == 0 {
			return errHostnameEmptyLabel
		}
		if len(label) > maxLabelLen {
			return errHostnameLabelTooLong
		}
		for j := 0; j < len(label); j++ {
			if !isLDHByte(label[j]) {
				return errHostnameInvalidChar
			}
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return errHostnameHyphenEdge
		}
		lastLabelStart = start
		start = i + 1
	}

	if !isValidTLDLabel(trimmed[lastLabelStart:]) {
		return errHostnameInvalidTLD
	}

	return nil
}

func isLDHByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-'
}

func isAlphaByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// JSON escaping can expand a byte up to 6x, so 128 keeps the whole log line
// under ~1 KiB even for a value made entirely of control bytes.
const maxLoggedValueLen = 128

// truncateForLog must not escape: zerolog's Str already escapes for JSON, and
// a second pass doubles every backslash. A rune split at the cut is safe too.
func truncateForLog(value string) string {
	if len(value) > maxLoggedValueLen {
		return value[:maxLoggedValueLen]
	}
	return value
}

// canonicalHostname expects a value that already passed hostnameError.
func canonicalHostname(value string) string {
	return strings.ToLower(strings.TrimSuffix(value, "."))
}

// A punycode TLD is accepted by prefix only; its contents are not decoded.
func isValidTLDLabel(label string) bool {
	if len(label) >= 4 && strings.EqualFold(label[:4], "xn--") {
		return true
	}
	if len(label) < 2 {
		return false
	}
	for i := 0; i < len(label); i++ {
		if !isAlphaByte(label[i]) {
			return false
		}
	}
	return true
}
