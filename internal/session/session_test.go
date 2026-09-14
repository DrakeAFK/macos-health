package session

import (
	"context"
	"encoding/json"
	"github.com/drakeafk/macos-health/internal/stats"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordingRoundTripAndLimits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.ndjson")
	r, e := Create(path)
	if e != nil {
		t.Fatal(e)
	}
	s := stats.Snapshot{SchemaVersion: 2, SampledAt: time.Now()}
	if e = r.Write(s); e != nil {
		t.Fatal(e)
	}
	r.written = MaxBytes
	if e = r.Write(s); e == nil {
		t.Fatal("recording limit ignored")
	}
	r.Close()
	if _, e = Create(path); e == nil {
		t.Fatal("existing recording replaced")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("recording is not private")
	}
	f, _ := os.Open(path)
	defer f.Close()
	replay := NewReplay(f)
	got, e := replay.Collect(context.Background())
	if e != nil || !got.SampledAt.Equal(s.SampledAt) {
		t.Fatalf("%+v %v", got, e)
	}
	if _, e = replay.Collect(context.Background()); e != io.EOF {
		t.Fatalf("end=%v", e)
	}
}
func TestReplayRejectsMalformedFutureAndNonIncreasing(t *testing.T) {
	for _, text := range []string{`garbage`, `{"schema_version":99}`, `{"schema_version":2}`, strings.Repeat("x", MaxFrame+1)} {
		if _, e := NewReplay(strings.NewReader(text)).Collect(context.Background()); e == nil {
			t.Fatal("invalid frame accepted")
		}
	}
	s := stats.Snapshot{SchemaVersion: 2, SampledAt: time.Now()}
	b, _ := json.Marshal(s)
	r := NewReplay(strings.NewReader(string(b) + "\n" + string(b)))
	if _, e := r.Collect(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, e := r.Collect(context.Background()); e == nil {
		t.Fatal("duplicate timestamp accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := NewReplay(strings.NewReader(string(b))).Collect(ctx); e == nil {
		t.Fatal("cancellation ignored")
	}
}
