//go:build darwin

package stats

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/load"
)

func loadAvg(ctx context.Context) (float64, float64, float64, error) {
	average, err := load.AvgWithContext(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("load average: %w", err)
	}
	if invalidLoad(average.Load1) || invalidLoad(average.Load5) || invalidLoad(average.Load15) {
		return 0, 0, 0, fmt.Errorf("load average returned an invalid value")
	}
	return average.Load1, average.Load5, average.Load15, nil
}

func parseLoadAvg(value string) (float64, float64, float64, error) {
	value = strings.Trim(strings.TrimSpace(value), "{}")
	parts := strings.Fields(value)
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("load average: expected 3 values, got %d", len(parts))
	}

	parsed := make([]float64, 3)
	for i, part := range parts {
		value, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("load average value %q: %w", part, err)
		}
		if invalidLoad(value) {
			return 0, 0, 0, fmt.Errorf("load average value %q is out of range", part)
		}
		parsed[i] = value
	}
	return parsed[0], parsed[1], parsed[2], nil
}

func invalidLoad(value float64) bool {
	return value < 0 || math.IsNaN(value) || math.IsInf(value, 0)
}
