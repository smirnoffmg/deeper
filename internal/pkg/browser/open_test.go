package browser

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandFor_Darwin(t *testing.T) {
	name, args, err := commandFor("darwin", "/tmp/report.html")
	assert.NoError(t, err)
	assert.Equal(t, "open", name)
	assert.Equal(t, []string{"/tmp/report.html"}, args)
}

func TestCommandFor_Linux(t *testing.T) {
	name, args, err := commandFor("linux", "/tmp/report.html")
	assert.NoError(t, err)
	assert.Equal(t, "xdg-open", name)
	assert.Equal(t, []string{"/tmp/report.html"}, args)
}

func TestCommandFor_Windows(t *testing.T) {
	name, args, err := commandFor("windows", "C:\\tmp\\report.html")
	assert.NoError(t, err)
	assert.Equal(t, "rundll32", name)
	assert.Equal(t, []string{"url.dll,FileProtocolHandler", "C:\\tmp\\report.html"}, args)
}

func TestCommandFor_UnsupportedOS(t *testing.T) {
	_, _, err := commandFor("plan9", "/tmp/report.html")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedOS))
}
