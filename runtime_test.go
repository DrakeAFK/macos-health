package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drakeafk/macos-health/internal/stats"
)

func TestMachineModes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	for _, args := range [][]string{{"--schema"}, {"--demo", "--json"}, {"--demo", "--prometheus"}, {"--demo", "--stream", "--samples", "2", "--interval", "250ms"}} {
		var out, errors bytes.Buffer
		args = append(args, "--config", path)
		if code := run(args, strings.NewReader(""), &out, &errors, false); code != 0 {
			t.Fatalf("%v: %d %s", args, code, errors.String())
		}
		if out.Len() == 0 {
			t.Fatal("empty export")
		}
		if args[0] == "--schema" {
			var schema map[string]any
			if e := json.Unmarshal(out.Bytes(), &schema); e != nil || schema["properties"] == nil {
				t.Fatal("invalid schema")
			}
		}
	}
	for _, h := range []struct {
		status string
		code   int
	}{{"healthy", 0}, {"partial", 1}, {"warning", 3}, {"critical", 4}} {
		if healthExit(stats.Health{Status: h.status}) != h.code {
			t.Fatal("health exit status")
		}
	}
}
func TestCLIRejectsConflictingAndUnboundedOptions(t *testing.T) {
	for _, args := range [][]string{{"--samples", "-1"}, {"--duration", "-1s"}, {"--theme", "bad"}, {"--page", "8"}, {"--stream", "--prometheus"}, {"--record", "a", "--replay", "b"}} {
		args = append(args, "--config", filepath.Join(t.TempDir(), "config.json"))
		if _, err := parseOptions(args, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
func TestConfigFlagsOverrideSavedPreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	var out bytes.Buffer
	if code := run([]string{"--config", path, "--theme", "amber", "--interval", "2s", "--save-config"}, nil, &out, io.Discard, false); code != 0 {
		t.Fatal(code)
	}
	o, e := parseOptions([]string{"--config", path, "--theme", "violet"}, io.Discard)
	if e != nil || o.theme != "violet" || o.interval != 2*time.Second {
		t.Fatalf("%+v %v", o, e)
	}
}

type addressWriter struct{ ch chan string }

func (w addressWriter) Write(b []byte) (int, error) {
	if strings.HasPrefix(string(b), "Serving local snapshots at ") {
		w.ch <- strings.TrimSpace(strings.TrimPrefix(string(b), "Serving local snapshots at "))
	}
	return len(b), nil
}
func TestLocalServerLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addresses := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, &pipeline{source: &demoCollector{}}, options{serve: "127.0.0.1:0", interval: 250 * time.Millisecond}, addressWriter{addresses})
	}()
	var address string
	select {
	case address = <-addresses:
	case <-time.After(3 * time.Second):
		t.Fatal("server startup timeout")
	}
	client := http.Client{Timeout: time.Second}
	for _, path := range []string{"/snapshot", "/metrics", "/healthz"} {
		resp, err := client.Get(address + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 && resp.StatusCode != 503 {
			t.Fatalf("%s: %d", path, resp.StatusCode)
		}
		if len(body) == 0 {
			t.Fatal("empty endpoint")
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
	if err := serve(context.Background(), &demoCollector{}, options{serve: "0.0.0.0:9797"}, io.Discard); err == nil {
		t.Fatal("nonlocal bind allowed")
	}
}
