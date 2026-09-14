package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"

	"github.com/drakeafk/macos-health/internal/app"
	"github.com/drakeafk/macos-health/internal/config"
	"github.com/drakeafk/macos-health/internal/session"
	"github.com/drakeafk/macos-health/internal/stats"
	"github.com/drakeafk/macos-health/internal/ui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type options struct {
	systemProcesses bool
	ports           bool
	once            bool
	json            bool
	interval        time.Duration
	noColor         bool
	noAltScreen     bool
	ascii           bool
	redact          bool
	version         bool
	stream          bool
	samples         int
	duration        time.Duration
	record          string
	replay          string
	sensors         bool
	theme           string
	page            int
	group           bool
	configPath      string
	saveConfig      bool
	schema          bool
	prometheus      bool
	check           bool
	serve           string
	demo            bool
}

func main() {
	interactive := isTerminal(os.Stdin) && isTerminal(os.Stdout)
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, interactive))
}

func run(args []string, input io.Reader, output, errOutput io.Writer, interactive bool) int {
	opts, err := parseOptions(args, errOutput)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(errOutput, "error:", err)
		return 2
	}
	if opts.version {
		fmt.Fprintln(output, versionString())
		return 0
	}
	if opts.schema {
		if err := json.NewEncoder(output).Encode(stats.JSONSchema()); err != nil {
			return 1
		}
		return 0
	}
	if opts.saveConfig {
		if err := config.Save(opts.configPath, opts.settings()); err != nil {
			fmt.Fprintln(errOutput, err)
			return 1
		}
		fmt.Fprintln(output, "Saved preferences to", opts.configPath)
		return 0
	}
	if runtime.GOOS != "darwin" && opts.replay == "" && !opts.demo {
		fmt.Fprintln(errOutput, stats.ErrUnsupportedPlatform)
		return 1
	}
	if os.Getenv("NO_COLOR") != "" {
		opts.noColor = true
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		opts.once = true
		opts.noColor = true
		opts.ascii = true
	}
	if !interactive {
		opts.once = true
		opts.noColor = true
	}
	if opts.json || opts.prometheus || opts.check {
		opts.once = true
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if opts.duration > 0 {
		var stop context.CancelFunc
		ctx, stop = context.WithTimeout(ctx, opts.duration)
		defer stop()
	}
	var source app.SnapshotCollector
	var closeSource func()
	switch {
	case opts.replay != "":
		f, e := os.Open(opts.replay)
		if e != nil {
			fmt.Fprintln(errOutput, e)
			return 1
		}
		source = session.NewReplay(f)
		closeSource = func() { _ = f.Close() }
	case opts.demo:
		source = &demoCollector{}
	default:
		c := stats.NewCollectorWithOptions(stats.CollectorOptions{Sensors: opts.sensors, Ports: opts.ports, SystemProcesses: opts.systemProcesses})
		source = c
		closeSource = c.Close
	}
	defer func() {
		cancel()
		if closeSource != nil {
			closeSource()
		}
	}()
	var recorder *session.Recorder
	if opts.record != "" {
		recorder, err = session.Create(opts.record)
		if err != nil {
			fmt.Fprintln(errOutput, "record:", err)
			return 1
		}
		defer recorder.Close()
	}
	collector := &pipeline{source: source, record: recorder, redact: opts.redact, replay: opts.replay != ""}
	if opts.serve != "" {
		if err := serve(ctx, collector, opts, errOutput); err != nil {
			fmt.Fprintln(errOutput, err)
			return 1
		}
		return 0
	}
	if opts.stream {
		return stream(ctx, collector, opts, output, errOutput)
	}
	if opts.once {
		var snapshot stats.Snapshot
		if opts.replay != "" {
			snapshot, err = collector.Collect(ctx)
		} else {
			snapshot, err = collectOne(ctx, collector)
		}
		if snapshot.SampledAt.IsZero() {
			fmt.Fprintln(errOutput, "error:", err)
			return 1
		}
		if e := writeSnapshot(output, snapshot, opts); e != nil {
			fmt.Fprintln(errOutput, e)
			return 1
		}
		if err != nil {
			fmt.Fprintln(errOutput, "warning:", err)
			return 1
		}
		if opts.check {
			return healthExit(snapshot.Health)
		}
		return 0
	}
	model := app.NewModel(app.Config{Context: ctx, Collector: collector, Interval: opts.interval, Timeout: 3 * time.Second, NoColor: opts.noColor, ASCII: opts.ascii, Redact: opts.redact, Theme: opts.theme, Page: ui.Page(opts.page), Group: opts.group, Replay: opts.replay != "", Save: func(page ui.Page, theme string, redact, group bool) error {
		saved := opts.settings()
		saved.Page = int(page)
		saved.Theme = theme
		saved.Redact = redact
		saved.Group = group
		return config.Save(opts.configPath, saved)
	}})
	programOptions := []tea.ProgramOption{tea.WithContext(ctx), tea.WithInput(input), tea.WithOutput(output)}
	if !opts.noAltScreen {
		programOptions = append(programOptions, tea.WithAltScreen())
	}
	_, err = tea.NewProgram(model, programOptions...).Run()
	if err != nil && ctx.Err() == nil {
		fmt.Fprintln(errOutput, "error:", err)
		return 1
	}
	return 0
}

func (o options) settings() config.Settings {
	return config.Settings{Interval: o.interval.String(), Theme: o.theme, Page: o.page, Redact: o.redact, ASCII: o.ascii, NoColor: o.noColor, Sensors: o.sensors, Ports: o.ports, SystemProcesses: o.systemProcesses, Group: o.group}
}
func healthExit(h stats.Health) int {
	switch h.Status {
	case "critical":
		return 4
	case "warning":
		return 3
	case "healthy":
		return 0
	default:
		return 1
	}
}
func writeSnapshot(w io.Writer, s stats.Snapshot, o options) error {
	if o.redact {
		s = s.Redacted()
	}
	if o.prometheus {
		_, err := io.WriteString(w, stats.Prometheus(s))
		return err
	}
	if o.json || o.stream {
		e := json.NewEncoder(w)
		e.SetEscapeHTML(false)
		if !o.stream {
			e.SetIndent("", "  ")
		}
		return e.Encode(s)
	}
	_, err := fmt.Fprintln(w, ui.RenderText(s))
	return err
}

func collectOne(ctx context.Context, collector app.SnapshotCollector) (stats.Snapshot, error) {
	first, firstErr := collectWithTimeout(ctx, collector, 3*time.Second)
	if firstErr != nil && first.SampledAt.IsZero() {
		return first, firstErr
	}

	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return first, ctx.Err()
	case <-timer.C:
	}

	second, secondErr := collectWithTimeout(ctx, collector, 3*time.Second)
	if !first.SampledAt.IsZero() {
		second = stats.MergeLastGood(second, first)
	}
	second.Health = stats.Assess(second, []stats.Snapshot{first})

	var coreErrors []string
	for _, metric := range []struct {
		name   string
		status stats.MetricStatus
	}{
		{"cpu", second.CPU.Status},
		{"memory", second.Memory.Status},
		{"disk", second.Disk.Status},
	} {
		if !metric.status.Available {
			coreErrors = append(coreErrors, metric.name)
		}
	}
	if len(coreErrors) > 0 {
		return second, fmt.Errorf("core telemetry unavailable: %s", strings.Join(coreErrors, ", "))
	}
	return second, secondErr
}

func collectWithTimeout(parent context.Context, collector app.SnapshotCollector, timeout time.Duration) (stats.Snapshot, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	return collector.Collect(ctx)
}

func parseOptions(args []string, output io.Writer) (options, error) {
	var opts options
	opts.configPath = config.Path()
	// Load defaults first, then let every explicit flag override them.
	for i, arg := range args {
		if strings.HasPrefix(arg, "--config=") {
			opts.configPath = strings.TrimPrefix(arg, "--config=")
		}
		if arg == "--config" && i+1 < len(args) {
			opts.configPath = args[i+1]
		}
	}
	defaults, err := config.Load(opts.configPath)
	if err != nil {
		return opts, fmt.Errorf("config: %w", err)
	}
	interval, _ := time.ParseDuration(defaults.Interval)
	set := flag.NewFlagSet("macos-health", flag.ContinueOnError)
	set.SetOutput(output)
	set.BoolVar(&opts.once, "once", false, "print one snapshot and exit")
	set.BoolVar(&opts.json, "json", false, "print one JSON snapshot and exit")
	set.DurationVar(&opts.interval, "interval", interval, "dashboard refresh interval (250ms to 1m)")
	set.BoolVar(&opts.noColor, "no-color", defaults.NoColor, "disable ANSI colors")
	set.BoolVar(&opts.noAltScreen, "no-alt-screen", false, "render without the terminal alternate screen")
	set.BoolVar(&opts.ascii, "ascii", defaults.ASCII, "use ASCII-only charts and symbols")
	set.BoolVar(&opts.redact, "redact", defaults.Redact, "hide computer name, hostname, and local addresses")
	set.BoolVar(&opts.version, "version", false, "print version information and exit")
	set.BoolVar(&opts.stream, "stream", false, "stream compact NDJSON snapshots until interrupted")
	set.IntVar(&opts.samples, "samples", 0, "stop a stream after N samples (0 = unlimited)")
	set.DurationVar(&opts.duration, "duration", 0, "stop collection after this duration")
	set.StringVar(&opts.record, "record", "", "record NDJSON to a new private file (256 MiB cap)")
	set.StringVar(&opts.replay, "replay", "", "replay NDJSON; --interval controls playback cadence")
	set.BoolVar(&opts.systemProcesses, "system-processes", defaults.SystemProcesses, "use ps for wider system process visibility; costs more and lacks footprint/I/O")
	set.BoolVar(&opts.ports, "ports", defaults.Ports, "inspect local TCP listeners (lsof, cached for 10s)")
	set.BoolVar(&opts.sensors, "sensors", defaults.Sensors, "read native IOReport/SMC silicon sensors; disable with =false")
	set.StringVar(&opts.theme, "theme", defaults.Theme, "ocean, amber or violet")
	set.IntVar(&opts.page, "page", defaults.Page, "initial page: 0 overview through 7 workloads")
	set.BoolVar(&opts.group, "group", defaults.Group, "group processes by owning application")
	set.StringVar(&opts.configPath, "config", opts.configPath, "preferences JSON file")
	set.BoolVar(&opts.saveConfig, "save-config", false, "save these preferences and exit")
	set.BoolVar(&opts.schema, "schema", false, "print the snapshot JSON Schema and exit")
	set.BoolVar(&opts.prometheus, "prometheus", false, "print Prometheus metrics and exit")
	set.BoolVar(&opts.check, "check", false, "health exit code: 0 healthy, 1 incomplete, 3 warning, 4 critical")
	set.StringVar(&opts.serve, "serve", "", "serve /snapshot, /metrics and /healthz on a loopback address")
	set.BoolVar(&opts.demo, "demo", false, "use synthetic telemetry for demos without exposing host data")
	set.Usage = func() {
		fmt.Fprintln(output, "macos-health — an Apple-silicon-first macOS health dashboard")
		fmt.Fprintln(output, "\nUsage: macos-health [options]\n\nOptions:")
		set.PrintDefaults()
	}
	if err := set.Parse(args); err != nil {
		return opts, err
	}
	if set.NArg() != 0 {
		return opts, fmt.Errorf("unexpected argument %q", set.Arg(0))
	}
	if opts.interval < 250*time.Millisecond || opts.interval > time.Minute {
		return opts, fmt.Errorf("--interval must be between 250ms and 1m")
	}
	if err := opts.settings().Validate(); err != nil {
		return opts, err
	}
	if opts.samples < 0 || opts.duration < 0 {
		return opts, fmt.Errorf("samples and duration cannot be negative")
	}
	if opts.samples > 0 || opts.duration > 0 {
		opts.stream = true
	}
	if opts.stream && opts.prometheus {
		return opts, fmt.Errorf("use --serve for repeated Prometheus scrapes")
	}
	if opts.serve != "" && (opts.replay != "" || opts.check || opts.prometheus) {
		return opts, fmt.Errorf("--serve cannot be combined with replay, check or prometheus")
	}
	if opts.replay != "" && opts.record != "" {
		return opts, fmt.Errorf("replay and recording cannot be combined")
	}
	return opts, nil
}

func versionString() string {
	return fmt.Sprintf("macos-health %s (commit %s, built %s)", version, commit, date)
}

func isTerminal(file *os.File) bool {
	return term.IsTerminal(file.Fd())
}
