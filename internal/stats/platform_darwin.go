//go:build darwin

package stats

import "errors"

var ErrUnsupportedPlatform = errors.New("macos-health requires macOS")
