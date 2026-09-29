package entities

import (
	"io"
	"os"
	"testing"

	"github.com/rs/zerolog/log"
)

// Without silencing the rollback log, the 64 KiB benchmark and tail-latency
// gate would measure logging I/O instead of classification. captureLog
// overrides this per test, which is safe because no test here runs t.Parallel.
func TestMain(m *testing.M) {
	log.Logger = log.Output(io.Discard)
	os.Exit(m.Run())
}
