//go:build darwin && cgo

package stats

/*
#cgo LDFLAGS: -framework CoreFoundation -framework IOKit
#include "silicon_native.h"
*/
import "C"

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

type sensorReader struct {
	samples     int
	mu          sync.Mutex
	native      *C.mh_native
	initialized bool
	last        time.Time
	cached      SiliconStats
}

func (r *sensorReader) sample(ctx context.Context, now time.Time) SiliconStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := SiliconStats{Enabled: true, Source: "IOReport + AppleSMC (private, best effort)"}
	if err := ctx.Err(); err != nil {
		result.Status = unavailableStatus(now, err)
		return result
	}
	if runtime.GOARCH != "arm64" {
		result.Status = unavailableStatus(now, fmt.Errorf("silicon sensors require a native Apple silicon build"))
		return result
	}
	if !r.last.IsZero() && now.Sub(r.last) < time.Second && r.samples >= 2 {
		return r.cached
	}
	if !r.initialized {
		r.native = C.mh_open()
		r.initialized = true
	}
	sampleAt := time.Now()
	raw := C.mh_sample(r.native, C.double(sampleAt.Sub(r.last).Seconds()))
	r.last = sampleAt
	r.samples++
	value := func(n C.double, flag uint) SensorValue {
		return SensorValue{Available: uint(raw.flags)&flag != 0, Value: float64(n)}
	}
	result.CPUWatts = value(raw.cpu_w, 1)
	result.GPUWatts = value(raw.gpu_w, 2)
	result.ANEWatts = value(raw.ane_w, 4)
	result.GPUPercent = value(raw.gpu_pct, 8)
	result.CPUTemperature = value(raw.cpu_t, 16)
	result.GPUTemperature = value(raw.gpu_t, 32)
	for i := 0; i < int(raw.fan_count); i++ {
		result.FanRPM = append(result.FanRPM, float64(raw.fans[i]))
	}
	if raw.flags != 0 || raw.fan_count > 0 {
		result.Status = availableStatus(now)
	} else {
		result.Status = unavailableStatus(now, fmt.Errorf("sensor channels unavailable or warming up"))
	}
	r.cached = result
	return result
}
func (r *sensorReader) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	C.mh_close(r.native)
	r.native = nil
	r.initialized = false
	r.samples = 0
	r.last = time.Time{}
}
