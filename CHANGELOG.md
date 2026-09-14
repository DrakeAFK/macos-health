# Changelog 

This file documents notable changes to `macos-health` 

Versioning is loosely inspired by Semantic Versioning
([https://semver.org](https://semver.org)), but without pretending everything is stable yet 

--- 

## [Unreleased]

Major dashboard release focused on diagnosis, local AI/developer workloads,
native Apple silicon telemetry, and agent-friendly exports.

### Added

* Eight responsive dashboard pages: Overview, Processes, Battery, System,
  Silicon, History, Insights, and Workloads
* Native Darwin `libproc` process collection with exact start identity, parent
  PID, physical footprint, and per-process disk I/O
* Search, app grouping, AI/Dev workload labels, PID-reuse-safe process details,
  and process history
* Read-only IOReport/AppleSMC/HID bridge for Apple silicon CPU/GPU/ANE power,
  GPU activity, temperatures, and fan RPM where channels exist
* Bounded in-memory history, transition events, evidence-backed insights, and
  model-weight memory budget estimates
* Private NDJSON recording/replay with a 256 MiB cap and strict monotonic
  timestamps
* `--stream`, `--samples`, `--duration`, `--record`, `--replay`, `--check`,
  `--schema`, `--prometheus`, `--serve`, and safe `--demo` modes
* Loopback-only JSON/Prometheus/health HTTP endpoints with bounded timeouts
* Strict private preferences at `$XDG_CONFIG_HOME/macos-health/config.json` or
  `~/.config/macos-health/config.json`
* Optional Developer ID signing and notarization hooks for release runners
* Full Bash, Fish, and Zsh completion coverage for the expanded CLI
* Health assessment with explainable warning/critical reasons instead of an
  opaque score
* Rolling CPU, memory, and battery sparklines
* Real kernel memory-pressure state and available headroom
* Compressed memory and active page-out throughput
* Apple silicon Performance/Efficiency core topology and per-core activity
* Battery maximum capacity, condition, cycles, temperature, Low Power Mode, and
  charger details when macOS exposes them
* Coarse permissionless macOS thermal/performance state
* Process PID and resident-memory reporting
* `--once`, `--json`, `--interval`, `--no-color`, `--no-alt-screen`, `--ascii`,
  `--redact`, and `--version`
* Top-level JSON `schema_version` for pipeline compatibility checks
* Automatic plain one-shot output for pipes and `TERM=dumb`
* Per-metric availability, sample time, error, and stale-value metadata
* Privacy redaction toggle (`x`) for host and local-address details
* Tests for parsers, counter resets, PID reuse, health rules, single-flight app
  state, terminal sanitization, Unicode, and responsive viewport bounds
* CI, Dependabot, Make targets, GoReleaser arm64/x86_64 archives, checksums,
  contribution guidance, security policy, and metric provenance documentation

### Changed

* Collection is stateful, concurrent, cadence-aware, and strictly single-flight
* Process CPU now uses interval deltas rather than a lifetime average
* System CPU uses per-core deltas and explicitly warms up on the first sample
* APFS usage now reflects shared container consumption rather than only the
  sealed `/` snapshot
* Internal-disk read/write throughput uses interval deltas when Darwin exposes
  drive counters
* Network selection follows observed/default-route activity and safely handles
  VPNs, counter resets, interface changes, and sleep gaps
* Slow-changing hardware, disk, thermal, battery-health, and process readings
  use separate cadences to reduce self-overhead
* External Apple tools are invoked by absolute path with a fixed locale
* External text is stripped of terminal and bidirectional control characters
* Last-good readings survive partial failures and are visibly marked stale
* The default overview is bounded at every terminal size, including 80×24
* Human-readable binary units are labeled KiB/MiB/GiB/TiB
* Source baseline is Go 1.25 with CGO-enabled Darwin releases targeting
  macOS 12 and newer

### Removed

* The committed, stale, arm64-only executable; release artifacts are generated
  from tags instead
* Invented Low/Medium/High “memory pressure percentage” thresholds
* Repeated full host/process scans on every one-second refresh

### Breaking

* The interactive layout and keyboard model have been redesigned
* JSON output uses a new nested, explicitly versioned-by-release snapshot shape
* Human-readable byte units now use binary labels

---

## [0.2.0] – 2025-12-21

Expanded system visibility and improved robustness of data collection and UI refresh behavior.

### Added

* Host system information section, including:
  * Device (computer) name
  * Hostname
  * macOS version and build
  * CPU model and core count
  * GPU model
  * Power source (AC or battery)
* Separate process lists for:
  * Top CPU-consuming processes
  * Top memory-consuming processes
* macOS memory pressure percentage alongside pressure category
* Best-effort handling for slow or partial system calls without losing the entire refresh

### Changed

* Process reporting split into distinct CPU-heavy and memory-heavy sections
* Data collection made fully best-effort so partial timeouts no longer blank the UI
* UI refresh logic fixed to reliably update once per second
* Host and system identity data cached to reduce overhead

### Removed

* Wi-Fi SSID reporting, due to macOS privacy restrictions and inconsistent availability

## [0.1.0] – 2025-10-27

Initial release 

### Added 

* First usable version of `macos-health` 
* Terminal-based system health dashboard for macOS 
* Non-blocking TUI built with Bubble Tea 
* CPU usage calculated from deltas (no blocking sleeps) 
* Load average reporting (1m / 5m / 15m) 
* Memory usage with macOS-style memory pressure: 

  * Low (< 75%) 
  * Medium (75–90%) 
  * High (> 90%) 
* Swap usage reporting 
* Disk usage for the root volume 
* Network throughput for the active interface only 
* Battery percentage, charge state, and estimated time remaining 
* System uptime display 
* Top processes by CPU and memory usage 
* Context-aware data collection with timeouts so slow syscalls don’t freeze the UI 

Everything refreshes once per second 
If something stalls, it gets skipped 
