//go:build darwin && cgo

package stats

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNativeKernelCollectors(t *testing.T) {
	c := NewCollector()
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	rows, err := c.readProcesses(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range rows {
		if p.row.PID == int32(os.Getpid()) {
			found = true
			if p.row.StartedAt.IsZero() || !p.row.FootprintAvailable || p.row.RSSBytes == 0 || !p.cpuAvailable {
				t.Fatal("current process metadata incomplete")
			}
		}
	}
	if !found {
		t.Fatal("current process missing")
	}
	vm, err := nativeVMCounters()
	if err != nil || vm.PageSize == 0 || vm.PageSize&(vm.PageSize-1) != 0 {
		t.Fatalf("VM counters: %+v %v", vm, err)
	}
	network, err := nativeNetCounters(ctx)
	if err != nil || len(network) == 0 {
		t.Fatalf("network counters: %v", err)
	}
}
