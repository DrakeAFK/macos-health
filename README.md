# macos-health

An Apple-silicon-first terminal dashboard that answers three questions quickly:

1. Is my Mac healthy?
2. If not, why?
3. Which process is responsible?

`macos-health` combines real macOS memory pressure, per-core CPU activity,
APFS capacity, network throughput, battery health, thermal state, and current
process deltas in a responsive Bubble Tea interface. It is read-only, local,
and unprivileged: no daemon, no analytics, no network requests, and no surprise
`sudo` prompt.

```text
macos-health [OK] Your Mac looks healthy                         now
[1 Overview]  2 Processes  3 Battery  4 System

CPU     14%  ▁▂▂▃  load 2.1/11  thermal Nominal
MEM     68%  ▆▆▆▆  12.2 GiB/18.0 GiB  pressure Normal
BAT     82% Discharging 4h12m  health 91%
DISK    318.4 GiB free  69%
NET     en0  ↓2.1 MiB/s ↑340 KiB/s

CULPRITS
CPU  Code Helper (Renderer)  42%
MEM  Browser Helper          612 MiB
```

The overview always fits the terminal. At 80×24 it stays compact; wide
terminals gain two-column metric and process layouts; very small terminals get
a bounded status-first view instead of clipped panels.

## Why it is macOS-aware

- Uses the kernel's Normal / Warning / Critical memory-pressure state and
  headroom, plus compression, swap, and live page-outs.
- Understands Apple silicon core topology such as `5P + 6E`.
- Reports battery maximum capacity, condition, cycles, pack temperature,
  estimated runtime, charger state, and Low Power Mode.
- Measures the shared APFS container instead of undercounting the sealed system
  snapshot, with live internal-disk read/write throughput.
- Selects the active/default interface without assuming all traffic uses
  `en0`, including VPN and Ethernet setups.
- Calculates current process CPU from interval deltas. `100%` means one logical
  core; multi-core processes can exceed it.
- Treats warming, unavailable, stale, and successful zero values as different
  states. Failed telemetry never silently becomes `0`.
- Exposes coarse, permissionless macOS thermal/performance pressure without
  pretending privileged CPU/GPU sensors are available.

See [Metrics and data sources](docs/metrics.md) for exact sources, units,
cadences, thresholds, and limitations.

## Install

### Release archive

Download the `darwin_arm64` archive for Apple silicon or `darwin_x86_64` for an
Intel Mac from [GitHub Releases](https://github.com/DrakeAFK/macos-health/releases),
then verify it against `checksums.txt` from the same release.

```sh
tar -xzf macos-health_VERSION_darwin_arm64.tar.gz
mkdir -p "$HOME/.local/bin"
install -m 0755 macos-health "$HOME/.local/bin/macos-health"
```

Add `$HOME/.local/bin` to `PATH` if it is not already there.

Release binaries target macOS 12 or newer and do not require Go at runtime.
They are currently unsigned/not notarized, so review the release-integrity and
Gatekeeper notes in [SECURITY.md](SECURITY.md).

### Go install

Source builds require Go 1.25 or newer, Xcode Command Line Tools, and CGO.

```sh
xcode-select --install # only if Apple Command Line Tools are missing
go install github.com/drakeafk/macos-health@latest
```

### Build from source

```sh
git clone https://github.com/DrakeAFK/macos-health.git
cd macos-health
make build
./dist/macos-health
```

## Use

Run the interactive dashboard:

```sh
macos-health
```

Keyboard controls:

| Key | Action |
| --- | --- |
| `1`–`8`, `Tab`, `Shift+Tab` | Switch Overview, Processes, Battery, System, Silicon, History, Insights, and Workloads |
| `/`, `Enter`, `Esc` | Search processes, inspect the selected process, and return |
| `a` | Group processes by owning app |
| `s` | Sort processes by CPU or resident memory |
| `[` / `]` | Change history window from 1 minute to 1 hour |
| `t`, `w` | Change theme, save preferences |
| `Space` | Pause or resume refreshes |
| `r` | Refresh immediately |
| `x` | Toggle host/address privacy redaction |
| `?` | Show responsive help |
| `q`, `Ctrl+C` | Quit |

The default refresh interval is one second. Collection is single-flight: a
slow attempt is never overlapped by another, and an older result cannot replace
a newer snapshot.

### One-shot and JSON output

Plain output is useful for diagnostics, screen readers, issue reports, and
shell pipelines:

```sh
macos-health --once
macos-health --json | jq '.health, .memory, .battery'
macos-health --json --redact > health.json
```

JSON snapshots include a top-level `schema_version` for consumers that need a
stable compatibility check.

When stdout is not a terminal, `macos-health` automatically emits one plain
snapshot rather than trying to open an interactive TUI.

### Command-line options

```text
--once            print one text snapshot and exit
--json            print one structured JSON snapshot and exit
--stream          emit newline-delimited JSON snapshots until interrupted
--samples 10      stop a stream after 10 samples
--record FILE     record private NDJSON snapshots (new file, 256 MiB cap)
--replay FILE     replay a recording for debugging or demos
--serve IP:PORT   serve loopback /snapshot, /metrics, and /healthz endpoints
--check           return 0 healthy, 1 incomplete, 3 warning, or 4 critical
--schema          print the versioned snapshot JSON Schema
--prometheus      print fresh Prometheus metrics for one snapshot
--interval 2s     set refresh interval (250ms to 1m)
--no-color        disable ANSI colors
--no-alt-screen   keep dashboard output in the normal screen buffer
--ascii           use ASCII-only charts and symbols
--redact          hide computer name, hostname, and local addresses
--sensors         enable native Apple silicon power, temperature, GPU, and fan data
--ports           inspect local TCP listeners
--system-processes use ps for wider system visibility (more overhead)
--theme NAME      ocean, amber, or violet
--group           start with app grouping enabled
--save-config     save preferences to the config file and exit
--demo            synthetic safe telemetry for screenshots and demos
--version         print version, commit, and build date
```

`NO_COLOR` is honored. `TERM=dumb` automatically selects plain ASCII one-shot
output. Exit status is `0` on success, `1` when required telemetry or execution
fails, and `2` for invalid command-line usage.

For local captures, use the ignored `recordings/` directory:

```sh
mkdir -p recordings
macos-health --record recordings/slow-build.macos-health.ndjson
macos-health --replay recordings/slow-build.macos-health.ndjson
```

## Dashboard pages

### Overview

Health state and explanation, CPU/load/thermal trend, unified-memory context,
battery, APFS free space, network rates, and the leading CPU/memory culprits.
Warnings are explainable rules, not an opaque synthetic score.

### Processes

The default Apple-native collector uses `libproc` to show every accessible
process with process start identity, parent PID, resident memory, physical
footprint, per-process disk I/O, and interval CPU. `/` searches names, owning
apps, PID, and the `AI`/`Dev` workload labels. `a` groups helpers into apps;
`Enter` opens a PID-reuse-safe detail view with history. `--system-processes`
uses `/bin/ps` when wider system visibility matters more than footprint and I/O.

### Battery

Charge and ETA history, maximum capacity, macOS condition, cycle count,
battery-pack temperature when available, active power source, adapter wattage,
and Low Power Mode.

### System

Mac model, SoC, P/E topology, GPU identity/core count, macOS build, uptime,
architecture/Rosetta state, APFS capacity, active interface, hostname, and local
addresses. Identity and address details live here rather than on the
screenshot-friendly overview.

### Silicon

On Apple silicon builds with CGO, the read-only native backend samples IOReport
energy and GPU residency plus AppleSMC/HID temperatures and fan RPM. It reports
CPU/GPU/Neural Engine power, GPU activity, and temperatures when the hardware
exposes them. Missing channels remain unavailable and never become zero.

### History, Insights, and Workloads

History keeps a bounded in-memory timeline with CPU, memory, GPU, network, and
disk charts. Insights records condition transitions and pairs warnings with
evidence and next actions. Workloads gives developers and local-AI users memory
budgets for model weights and shows grouped development/AI processes. `--record`
and `--replay` make slowdowns reproducible without a daemon or network service.

### Local API

`--serve 127.0.0.1:9797` exposes the latest redacted-or-unredacted snapshot as
JSON, Prometheus text, and a health endpoint. It binds only to a literal
loopback address and uses bounded HTTP timeouts. The same schema is available
with `--schema` for agents and integrations.

## Trust, privacy, and permissions

All collection happens locally through Darwin system interfaces and Apple tools
invoked by absolute path with a fixed locale. The optional native Silicon page
uses read-only IOReport and AppleSMC/HID calls. Process arguments, environments,
battery serials, and hardware UUIDs are not collected. External text is
sanitized before terminal rendering.

The System page and exported snapshots can still contain a computer name,
hostname, local addresses, process names, and PIDs. Use `x` or `--redact`
before sharing output. Redaction does not upload, delete, or modify anything on
the Mac.

`macos-health` does not require root, Full Disk Access, Accessibility access,
or Location Services. It never launches `sudo` and does not make an outbound
network request.

## Deliberate limitations

Some Apple silicon power, temperature, fan, and residency channels are
available through the optional native backend. Coverage is model- and
macOS-dependent; unsupported channels are marked unavailable. The dashboard
never prompts for elevated privileges or invents a value.

Battery and thermal properties are best-effort because Apple can change their
shape between hardware and macOS releases. Unsupported readings are hidden or
marked unavailable. Intel Macs remain supported, but Apple-silicon-only fields
such as P/E topology naturally do not appear.

## Development

```sh
make check          # format check, vet, race-enabled tests, native build
make test
make build-all      # Darwin arm64 and x86_64
make release-check
make snapshot       # unpublished GoReleaser artifacts
```

The test suite covers malformed Apple command output, counter resets and PID
reuse, stale-value retention, terminal control characters, health thresholds,
model single-flight behavior, and UI bounds from tiny terminals through wide
layouts. CI runs on native Apple silicon and Intel macOS runners; tagged builds
produce architecture-specific archives and SHA-256 checksums.

Project structure:

```text
.
├── main.go                 CLI, one-shot/JSON output, TUI startup
├── internal/app            Bubble Tea state and single-flight scheduling
├── internal/stats          stateful Darwin collectors and health assessment
├── internal/ui             responsive pages, text output, and themes
├── docs/metrics.md         metric provenance and limitations
├── Makefile
└── .goreleaser.yaml
```

Read [CONTRIBUTING.md](CONTRIBUTING.md) before submitting changes. Report
security issues privately as described in [SECURITY.md](SECURITY.md).

## License

MIT — see [LICENSE](LICENSE).
