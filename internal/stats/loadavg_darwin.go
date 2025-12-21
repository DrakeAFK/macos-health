package stats

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
)

func loadAvg(ctx context.Context) (float64, float64, float64, error) {
	out, err := exec.CommandContext(ctx, "sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return 0, 0, 0, err
	}

	s := strings.TrimSpace(string(bytes.TrimSpace(out)))
	s = strings.Trim(s, "{}")
	parts := strings.Fields(s)
	if len(parts) < 3 {
		return 0, 0, 0, err
	}

	l1, _ := strconv.ParseFloat(parts[0], 64)
	l5, _ := strconv.ParseFloat(parts[1], 64)
	l15, _ := strconv.ParseFloat(parts[2], 64)
	return l1, l5, l15, nil
}
