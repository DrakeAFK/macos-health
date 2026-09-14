// Package session provides bounded, private NDJSON recordings and streaming replay.
package session

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/drakeafk/macos-health/internal/stats"
)

const MaxBytes int64 = 256 << 20
const MaxFrame = 8 << 20

type Recorder struct {
	mu      sync.Mutex
	file    *os.File
	written int64
}

func Create(path string) (*Recorder, error) {
	// Refuse replacement and symlink following; recordings may contain process names.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &Recorder{file: f}, nil
}
func (r *Recorder) Write(s stats.Snapshot) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.written+int64(len(b)) > MaxBytes {
		return fmt.Errorf("recording reached the 256 MiB limit")
	}
	n, err := r.file.Write(b)
	r.written += int64(n)
	return err
}
func (r *Recorder) Close() error { r.mu.Lock(); defer r.mu.Unlock(); return r.file.Close() }

type Replay struct {
	scanner  *bufio.Scanner
	bytes    int64
	previous stats.Snapshot
}

func NewReplay(r io.Reader) *Replay {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64<<10), MaxFrame)
	return &Replay{scanner: s}
}
func (r *Replay) Collect(ctx context.Context) (stats.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return stats.Snapshot{}, err
	}
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return stats.Snapshot{}, err
		}
		return stats.Snapshot{}, io.EOF
	}
	line := r.scanner.Bytes()
	r.bytes += int64(len(line) + 1)
	if r.bytes > MaxBytes {
		return stats.Snapshot{}, fmt.Errorf("replay exceeds 256 MiB")
	}
	var s stats.Snapshot
	if err := json.Unmarshal(line, &s); err != nil {
		return s, fmt.Errorf("invalid recording: %w", err)
	}
	if s.SchemaVersion < 1 || s.SchemaVersion > stats.SnapshotSchemaVersion {
		return s, fmt.Errorf("unsupported snapshot schema %d", s.SchemaVersion)
	}
	if s.SampledAt.IsZero() || (!r.previous.SampledAt.IsZero() && !s.SampledAt.After(r.previous.SampledAt)) {
		return s, fmt.Errorf("recording timestamps must increase")
	}
	if len(s.Processes.All) > 100000 {
		return s, fmt.Errorf("recording process table exceeds limit")
	}
	r.previous = stats.Snapshot{SampledAt: s.SampledAt}
	return s, nil
}
