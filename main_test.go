package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"
	"time"
)

func TestParseOptions(t *testing.T) {
	var output bytes.Buffer
	opts, err := parseOptions([]string{
		"--once", "--json", "--interval", "2s", "--no-color",
		"--no-alt-screen", "--ascii", "--redact",
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
	if !opts.once || !opts.json || !opts.noColor || !opts.noAltScreen || !opts.ascii || !opts.redact {
		t.Fatalf("parsed options = %+v", opts)
	}
	if opts.interval != 2*time.Second {
		t.Fatalf("interval = %v", opts.interval)
	}
}

func TestParseOptionsRejectsInvalidInput(t *testing.T) {
	for _, args := range [][]string{
		{"--interval", "100ms"},
		{"--interval", "2m"},
		{"unexpected"},
	} {
		var output bytes.Buffer
		if _, err := parseOptions(args, &output); err == nil {
			t.Fatalf("parseOptions(%q) succeeded", args)
		}
	}
}

func TestParseOptionsHelp(t *testing.T) {
	var output bytes.Buffer
	_, err := parseOptions([]string{"--help"}, &output)
	if err != flag.ErrHelp {
		t.Fatalf("error = %v, want flag.ErrHelp", err)
	}
	if !strings.Contains(output.String(), "Usage: macos-health") {
		t.Fatalf("help output missing usage: %q", output.String())
	}
}

func TestRunVersionWithoutTTY(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	version, commit, date = "v1.2.3", "abc123", "2026-07-09"
	defer func() { version, commit, date = oldVersion, oldCommit, oldDate }()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--version"}, strings.NewReader(""), &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d; stderr = %q", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "macos-health v1.2.3 (commit abc123, built 2026-07-09)" {
		t.Fatalf("version output = %q", got)
	}
}

func TestRunUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--interval", "1ms"}, strings.NewReader(""), &stdout, &stderr, false)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "between 250ms and 1m") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
