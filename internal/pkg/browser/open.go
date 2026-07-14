// Package browser opens a local file in the user's default web browser.
package browser

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
)

// ErrUnsupportedOS is returned when Open is called on a GOOS with no known
// way to launch the default browser.
var ErrUnsupportedOS = errors.New("unsupported OS for opening a browser")

// Open launches the OS default handler for the given local file path.
func Open(path string) error {
	name, args, err := commandFor(runtime.GOOS, path)
	if err != nil {
		return err
	}
	if err := exec.Command(name, args...).Start(); err != nil {
		return fmt.Errorf("failed to open %s: %w", path, err)
	}
	return nil
}

func commandFor(goos, path string) (string, []string, error) {
	switch goos {
	case "darwin":
		return "open", []string{path}, nil
	case "linux":
		return "xdg-open", []string{path}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", path}, nil
	default:
		return "", nil, fmt.Errorf("%w: %s", ErrUnsupportedOS, goos)
	}
}
