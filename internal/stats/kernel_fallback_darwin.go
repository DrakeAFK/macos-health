//go:build darwin && !cgo

package stats

import (
	"context"
	"fmt"
	gnet "github.com/shirou/gopsutil/v3/net"
	"time"
)

func nativeVMCounters() (vmCounters, error) {
	return vmCounters{}, fmt.Errorf("native VM counters require CGO")
}
func (c *Collector) readProcesses(ctx context.Context, now time.Time) ([]parsedProcess, error) {
	out, err := commandOutput(ctx, "/bin/ps", "-axo", "pid=,ppid=,etime=,time=,pmem=,rss=,comm=")
	if err != nil {
		return nil, err
	}
	return parseProcesses(string(out))
}

func nativeNetCounters(ctx context.Context) ([]gnet.IOCountersStat, error) {
	return gnet.IOCountersWithContext(ctx, true)
}
