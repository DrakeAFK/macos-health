# Metrics and data sources

`macos-health` is a local, best-effort view of macOS health. It favors Darwin
kernel interfaces and Apple tools, with an optional native IOReport/AppleSMC
backend for richer Apple-silicon telemetry. A missing reading is reported as
unavailable; it is never converted into a successful zero value.

## Sampling model

The interactive dashboard requests a base snapshot about once per second by
default. Collectors run concurrently under a bounded context, while expensive
or slow-changing values are cached at a lower cadence.

| Data | Update policy |
| --- | --- |
| CPU counters, load, memory, disk I/O, network counters, uptime | Every base snapshot |
| Native process list and per-process I/O/footprint | Every base snapshot, cached for 2 seconds in the TUI |
| TCP listeners (`--ports`) | At most every 10 seconds |
| Silicon energy, GPU residency, temperatures, fans | Once per second; native sensor channels are capability-tested |
| Battery state | At most every 5 seconds |
| Thermal pressure | At most every 5 seconds |
| Disk capacity | At most every 10 seconds |
| Default network route | At most every 10 seconds |
| Battery health details | At most every 30 seconds |
| Hardware and OS identity | At most every 24 hours |

The first CPU, process CPU, network-rate, and page-out-rate readings need two
samples. They are shown as warming up rather than as zero. When a later sample
fails, the application can retain the last successful value and mark it
stale. `sampled_at`, availability, staleness, and a sanitized error are carried
with each metric group.

Release binaries enable CGO. The Darwin CPU telemetry path and race-enabled
test suite are supported and shipped only with `CGO_ENABLED=1`.

## CPU and load

Sources:

- per-core CPU time counters through `gopsutil/cpu`;
- `vm.loadavg` through the Darwin `sysctl` interface.

CPU use is the non-idle delta between consecutive per-core counter samples.
Each core is clamped to 0–100%, and whole-system CPU is the average of the
cores, so the dashboard's aggregate remains in the 0–100% range. This is not a
clock-frequency, power, or performance-core residency measurement.

Load values are the macOS 1, 5, and 15 minute runnable-work averages. They are
not percentages and should be interpreted relative to workload and logical
core count.

## Memory

Sources:

- virtual memory and swap totals through `gopsutil/mem`;
- `kern.memorystatus_vm_pressure_level` and `kern.memorystatus_level` through
  `sysctl`;
- compressor occupancy and page-out counters from `host_statistics64` in CGO
  builds, with `/usr/bin/vm_stat` as the fallback.

Memory pressure is the kernel's Normal, Warning, or Critical status when that
interface is available. Available percent is the kernel memorystatus headroom
value. It is not a reconstruction of Activity Monitor's private graph.

macOS deliberately uses otherwise idle memory for caches and compression.
Consequently, used-memory percent by itself is not a health verdict. Compressor
occupancy, swap use, pressure state, and sustained page-outs provide the useful
context. Page-out throughput is a counter delta and is unavailable on its first
sample or after a counter reset.

## Disk capacity

Sources: filesystem statistics and Darwin drive counters through
`gopsutil/disk`, preferring `/System/Volumes/Data` and falling back to `/`.

APFS volumes share container capacity. Used space is calculated as total minus
available space so sibling volumes, system data, and APFS snapshots are
reflected. The value may therefore differ from a sum of visible files. Read and
write throughput are interval deltas from the internal `disk0` counters when
the CGO Darwin API exposes them. The first rate sample and counter resets are
unavailable rather than zero. This does not report SSD wear, SMART status, or
I/O latency.

## Network

Sources:

- interface metadata and byte counters through `gopsutil/net`;
- the default route from `/sbin/route -n get default`.

Rates are byte-counter deltas over elapsed wall time. Counter resets, newly
seen interfaces, or long sampling gaps cause rates to warm up again instead of
producing a spike. The dashboard chooses the busiest usable interface when
traffic is present and otherwise prefers the default-route interface. It does
not sum all interfaces, inspect packets, resolve remote hosts, or make a
network request.

Local non-loopback addresses are collected for display. Link-local and
unspecified addresses are excluded.

## Battery and power

Sources:

- current charge, charging state, power source, and estimated time from
  `/usr/bin/pmset -g batt`;
- health, cycle count, maximum capacity, and charger details from
  `/usr/sbin/system_profiler SPPowerDataType -json`;
- Low Power Mode from `/usr/bin/pmset -g custom`;
- best-effort battery-pack temperature from the `AppleSmartBattery` I/O
  Registry entry exposed by `/usr/sbin/ioreg`.

Time remaining is an estimate produced by macOS and may disappear while the
system learns a changing workload. Health details vary by Mac model and macOS
version. The displayed battery temperature, when available, is the pack
sensor; it is not a CPU, GPU, enclosure, or ambient temperature.

Desktops and systems without an internal battery report the battery as absent
rather than unhealthy.

## Thermal pressure

Source: `/usr/bin/pmset -g therm`.

The dashboard reports the system thermal/performance pressure state and any
CPU speed, scheduler, or thermal limits that macOS exposes. A nominal state
means macOS has not reported a warning through this interface; it does not mean
that every component is cool.

The optional Silicon page uses read-only IOReport and AppleSMC/HID interfaces.
These are private, model- and OS-dependent APIs; the backend is isolated behind
capability checks and can be disabled with `--sensors=false`. It may report:

- CPU, GPU, and Neural Engine energy in watts;
- GPU active residency;
- average CPU and GPU sensor temperatures;
- fan RPM where SMC exposes stable fan keys.

The following remain intentionally unsupported:

- CPU-cluster frequency and residency;
- Neural Engine and media-engine utilization;
- package-level energy attribution.

`macos-health` never invokes `sudo` or `powermetrics` and does not prompt for
privileged access to fill these gaps.

## Processes

The default source is Darwin `libproc` (`proc_listallpids`, task info, and
`proc_pid_rusage`). `--system-processes` selects `/bin/ps` for wider visibility
when needed.

Process CPU is calculated from cumulative-time deltas keyed by PID and exact
process start time, then lightly smoothed. A single process can exceed 100% by
using multiple cores; its upper bound is the logical core count multiplied by
100%. Memory ranking uses resident bytes; detail views also show physical
footprint and disk I/O rates when available. Only executable names, PIDs,
parent PIDs, and start times are retained, never arguments or environments.

macOS can hide or race process information as processes exit. Unreadable rows
are skipped, so the lists are useful rankings rather than an accounting ledger.

## Host and uptime

Sources:

- `sysctl` for machine, chip, architecture, core topology, and Rosetta state;
- `/usr/sbin/system_profiler` JSON for hardware and display identity;
- `/usr/sbin/scutil`, `/usr/bin/sw_vers`, and `os.Hostname` for local identity
  and the macOS version;
- `gopsutil/host` for uptime.

GPU identity describes installed or integrated graphics hardware. It does not
imply that GPU utilization telemetry is available.

## Health assessment

The health summary is explainable rule output, not a synthetic benchmark or a
medical-style diagnosis. Current rules include:

- kernel memory warning/critical pressure and page-outs at or above 10 MiB/s;
- disk warning at 90% used and critical status at 97%;
- thermal warnings reported by macOS;
- service battery condition or maximum capacity below 80%;
- low battery warnings at 20% and critical status at 10% while discharging;
- system CPU at or above 90% continuously for at least 10 seconds;
- unavailable or stale core telemetry.

Thresholds indicate conditions worth investigating. Short spikes and a single
unavailable collector are not proof of a hardware fault.

## Permissions and privacy

Metric collection is read-only and local. It requires neither root access nor
Full Disk Access, does not initiate outbound network connections, and is not
expected to trigger a macOS privacy-consent prompt. macOS permission or API
changes can still make an individual reading unavailable.

The normal dashboard can show computer name, hostname, local IP addresses,
process names, and PIDs. Treat screenshots and exported snapshots as potentially
sensitive. Use redacted output when sharing diagnostics; redaction removes host
identity, address, and listener values. Recordings are created with mode 0600,
refuse replacement of an existing file, and stop at 256 MiB. The local API
accepts only loopback binds and has no write endpoints. No telemetry is uploaded
by the application.
