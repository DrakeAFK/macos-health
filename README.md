# macos-health

A terminal monitor for macOS. CPU, memory pressure, battery, disk, network,
and processes in one place, with native Apple silicon power and temperature
readings where available.

Runs as your user. No daemon, account, telemetry, or outbound network requests.

```sh
git clone https://github.com/DrakeAFK/macos-health.git
cd macos-health
make build
./dist/macos-health
```

Requires macOS, Go 1.25 or newer, and Xcode Command Line Tools. Install the
command line tools with `xcode-select --install` if needed. Builds use CGO and
target macOS 12 or newer. Intel Macs work too; Apple silicon fields are omitted
when unavailable.

To install into your user directory:

```sh
make install PREFIX="$HOME/.local"
```

Add `$HOME/.local/bin` to `PATH` if needed. Build from source for now; packaged
releases are not yet published.

## What it shows

- **Overview:** CPU/load, memory pressure, battery, APFS free space, network
  rates, and the processes using the most CPU and memory.
- **Processes:** search, sort, group helpers by app, and inspect a process's
  history, memory footprint, and disk I/O.
- **Battery:** charge, estimated runtime, capacity, cycles, temperature, power
  source, and charger details.
- **System:** Mac model, chip, core topology, macOS version, uptime, disk,
  active interface, hostname, and local addresses.
- **Silicon:** CPU/GPU/Neural Engine power, GPU activity, temperatures, and fan
  speeds when the hardware exposes them.
- **History:** CPU, memory, GPU, network, and disk charts, held in memory.
- **Insights:** changes in health conditions, with the readings behind them.
- **Workloads:** grouped development/AI processes and model-weight memory
  estimates. Estimates are not a guarantee that a model will fit or run well.

Memory pressure comes from the kernel. Process CPU comes from interval deltas;
100% means one logical core, so a process can exceed it. Disk capacity reflects
the shared APFS container. Network rates follow the active interface, including
VPN and Ethernet connections.

Missing readings are marked unavailable. Rates need an initial sample before
they can be calculated; failed readings can retain a visibly stale value.
See [metrics and data sources](docs/metrics.md) for units, sampling intervals,
health thresholds, and hardware limitations.

## Controls

| Key | Action |
| --- | --- |
| `1`–`8`, `Tab`, `Shift+Tab` | Switch pages |
| `j` / `k`, `↓` / `↑`, `PgDown`, `PgUp` | Move through processes |
| `/`, `Enter`, `Esc` | Search, inspect a process, return |
| `a` | Group processes by app |
| `s` | Sort processes by CPU or resident memory |
| `[` / `]` | Change history window from 1 minute to 1 hour |
| `t` | Change theme |
| `w` | Save preferences |
| `Space` | Pause or resume refreshes |
| `r` | Refresh now |
| `x` | Toggle host/address redaction |
| `?` | Show help |
| `q`, `Ctrl+C` | Quit |

Refreshes default to one second. A new collection waits for the previous one
to finish. The layout adapts to terminal size, including 80×24.

## Shell use

```sh
macos-health --once
macos-health --json --redact | jq '.health, .memory, .battery'
macos-health --stream --samples 10
macos-health --check
```

Piped output defaults to one plain snapshot. `--json` selects JSON;
`--stream` selects newline-delimited JSON. Snapshots include `schema_version`.

Record and replay a session:

```sh
mkdir -p recordings
macos-health --record recordings/slow-build.macos-health.ndjson --redact
macos-health --replay recordings/slow-build.macos-health.ndjson
```

Recordings use new files with mode 0600 and stop at 256 MiB. `recordings/` is
ignored by Git.

For a local HTTP endpoint:

```sh
macos-health --serve 127.0.0.1:9797 --redact
```

This serves `/snapshot` (JSON), `/metrics` (Prometheus), and `/healthz` on a
literal loopback address only. Other local processes can read these endpoints.

## Options

```text
--once             print one text snapshot and exit
--json             print one JSON snapshot and exit
--stream           stream newline-delimited JSON until interrupted
--samples 10       stop a stream after 10 samples
--duration 30s     stop collection after 30 seconds
--record FILE      record snapshots to a new file
--replay FILE      replay a recording
--serve IP:PORT    serve local HTTP endpoints
--check            exit 0 healthy, 1 incomplete, 3 warning, 4 critical
--schema           print the snapshot JSON Schema
--prometheus       print Prometheus metrics for one snapshot
--interval 2s      set refresh interval (250ms to 1m)
--no-color         disable ANSI colors
--no-alt-screen    use the normal terminal screen buffer
--ascii            use ASCII charts and symbols
--redact           hide host identity, local addresses, and listener details
--sensors=false    disable native silicon sensors (enabled by default)
--ports            inspect local TCP listeners
--system-processes use ps for wider visibility; lacks footprint/I/O data
--theme NAME       ocean, amber, or violet
--page N           initial page: 0 Overview through 7 Workloads
--group            start with app grouping enabled
--config FILE      use a different preferences file
--save-config      save preferences and exit
--demo             use synthetic data
--version          print version, commit, and build date
--help             show CLI help
```

Preferences live at `$XDG_CONFIG_HOME/macos-health/config.json`, or
`~/.config/macos-health/config.json` when that variable is unset. Explicit flags
override saved values.

`NO_COLOR` disables colors. `TERM=dumb` selects plain ASCII one-shot output.
Normal exit status is 0 on success, 1 for collection/execution failure, or 2
for invalid usage; `--check` adds the health statuses listed above.

## Privacy and limits

Collection reads Darwin interfaces and Apple system tools. It does not require
root, Full Disk Access, or Accessibility permission, and never launches `sudo`.
Native sensors use private IOReport and AppleSMC APIs; availability varies by
Mac and macOS version. Disable them with `--sensors=false`.

Process arguments, environments, battery serials, and hardware UUIDs are not
retained in snapshots. Exported data can contain hostnames, local addresses,
process names, and PIDs. `--redact` removes host identity, addresses, and listener
details; process names and PIDs remain. Review output before sharing it.

The tool reads system state. It writes files only when saving preferences or
recording a session, and opens a listening socket only with `--serve`.

It does not report SSD wear, SMART status, CPU-cluster frequency, or Neural
Engine utilization. Battery estimates and sensor coverage depend on macOS.
Health warnings describe measured conditions; they do not diagnose hardware
faults. See [SECURITY.md](SECURITY.md) for security reports and release checks.

## Development

```sh
make check          # formatting, vet, race tests, native build
make build-all      # Darwin arm64 and x86_64
make vuln           # manual Go vulnerability scan; downloads the scanner
make release-check  # validate local GoReleaser configuration
make snapshot       # local archives and checksums; no publishing
```

Checks and packaging run locally. This repository does not use GitHub Actions
or scheduled Dependabot updates. GoReleaser is optional and only packages
archives; publishing is disabled in its configuration.

See [CONTRIBUTING.md](CONTRIBUTING.md) for development and manual release steps.

## License

[MIT](LICENSE), copyright Drake Hopkins. The separate
[macmon notice](docs/licenses/macmon-MIT.txt) credits the native sensor reference;
see [third-party acknowledgments](THIRD_PARTY_NOTICES.md).
