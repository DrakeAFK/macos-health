//go:build !darwin

package stats

import (
	"context"
	"errors"
)

var ErrUnsupportedPlatform = errors.New("macos-health requires macOS")

type Collector struct{}

func NewCollector() *Collector { return &Collector{} }

func (c *Collector) Collect(context.Context) (Snapshot, error) {
	return Snapshot{}, ErrUnsupportedPlatform
}

type CollectorOptions struct {
	SystemProcesses bool
	Sensors         bool
	Ports           bool
}

func NewCollectorWithOptions(CollectorOptions) *Collector { return NewCollector() }
func (*Collector) Close()                                 {}
