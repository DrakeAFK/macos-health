//go:build darwin

package stats

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func commandOutput(ctx context.Context, path string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return out, nil
}
