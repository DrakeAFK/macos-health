//go:build !darwin || !cgo

package stats

import (
	"context"
	"fmt"
	"time"
)

type sensorReader struct{}

func (*sensorReader) sample(_ context.Context, now time.Time) SiliconStats {
	return SiliconStats{Enabled: true, Status: unavailableStatus(now, fmt.Errorf("sensors require macOS and CGO"))}
}
func (*sensorReader) close() {}
