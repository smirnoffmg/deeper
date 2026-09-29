package entities

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// The longest hostname isDomain accepts: the worst case for its label loop.
var hostname253 = strings.Join([]string{
	strings.Repeat("a", 63),
	strings.Repeat("a", 63),
	strings.Repeat("a", 63),
	strings.Repeat("a", 61),
}, ".")

// Past the length cutoff, so its benchmark measures the cutoff path, not
// matcher cost.
var hostname64KiB = strings.Repeat("a", 64*1024)

func BenchmarkGuessTraceType253(b *testing.B) {
	if len(hostname253) != maxHostnameLen {
		b.Fatalf("test fixture: hostname253 is %d bytes, want %d", len(hostname253), maxHostnameLen)
	}
	b.ReportAllocs()
	for b.Loop() {
		guessTraceType(hostname253)
	}
}

func BenchmarkGuessTraceType64KiB(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		guessTraceType(hostname64KiB)
	}
}

// The success path must report 0 allocs/op.
func BenchmarkHostnameError(b *testing.B) {
	const value = "registry-one.example.com"
	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = hostnameError(value)
	}
	_ = err
}

const tailLatencySamples = 1000

const tailLatencyMaxP99 = 10 * time.Millisecond

// maxClassifiableLen is the worst case that still runs every matcher; 64 KiB
// confirms longer values take the cutoff path instead.
var tailLatencyLengths = map[string]int{
	"at_cutoff_1024_bytes": maxClassifiableLen,
	"64KiB":                64 * 1024,
}

// Letters alone are not representative: digits and hyphens pushed p99 well
// above the letters-only form.
func tailLatencyFormsAt(n int) map[string]string {
	return map[string]string{
		"letters":       repeatToLen("a", n),
		"digits":        repeatToLen("1", n),
		"dots":          repeatToLen(".", n),
		"mixed_ldh":     repeatToLen("a1-", n),
		"hostile_bytes": repeatToLen("\x00\xff\"\\\x01\x02", n),
	}
}

func repeatToLen(pattern string, n int) string {
	s := strings.Repeat(pattern, n/len(pattern)+1)
	return s[:n]
}

// Asserts p99 rather than the benchmark's mean, which once hid a tail twice as
// long. Wall-clock, so it is gated behind DEEPER_PERF=1 (make test-perf).
func TestGuessTraceType64KiBTailLatency(t *testing.T) {
	if os.Getenv("DEEPER_PERF") != "1" {
		t.Skip("skipped unless DEEPER_PERF=1 is set; run: DEEPER_PERF=1 go test -run TailLatency -count=1 -v ./internal/pkg/entities/ (or: make test-perf)")
	}

	lengthNames := make([]string, 0, len(tailLatencyLengths))
	for name := range tailLatencyLengths {
		lengthNames = append(lengthNames, name)
	}
	slices.Sort(lengthNames)

	for _, lengthName := range lengthNames {
		forms := tailLatencyFormsAt(tailLatencyLengths[lengthName])
		formNames := make([]string, 0, len(forms))
		for name := range forms {
			formNames = append(formNames, name)
		}
		slices.Sort(formNames)

		for _, formName := range formNames {
			value := forms[formName]
			t.Run(lengthName+"/"+formName, func(t *testing.T) {
				durations := make([]time.Duration, tailLatencySamples)
				for i := range durations {
					start := time.Now()
					guessTraceType(value)
					durations[i] = time.Since(start)
				}
				slices.Sort(durations)

				p50 := durations[tailLatencySamples*50/100]
				p99 := durations[tailLatencySamples*99/100]
				worst := durations[tailLatencySamples-1]
				t.Logf("p50=%s p99=%s max=%s", p50, p99, worst)

				if p99 > tailLatencyMaxP99 {
					t.Errorf("p99 = %s, want <= %s", p99, tailLatencyMaxP99)
				}
			})
		}
	}
}
