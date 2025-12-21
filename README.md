
# macos-health 

A terminal-based system health dashboard for macOS 

This exists because: 

* I wanted a fast, readable system overview 
* `top` is meh 
* `htop` is better, but not macOS-aware 
* I wanted to build something in Go and actually understand it 

So here we are 

--- 

## What it does 

`macos-health` shows you some stuff you might care about, in real time, without freezing your terminal or lying to you about memory 

It refreshes once per second and gives you a clean overview of system health 

--- 

## Features

* Non-blocking terminal UI (Bubble Tea)
* CPU usage (delta-based, no fake sleeps)
* Load average (1m / 5m / 15m)
* Memory usage **with macOS memory pressure**
* Swap usage
* Disk usage (root volume)
* Network throughput for the **active interface only**
* Battery percentage, state, and remaining time
* System uptime
* Host system information (device name, hostname, OS version/build, CPU/GPU, power source)
* Top processes by CPU **and** top processes by memory
* Context timeouts so one slow syscall doesn’t hang the UI

If something stalls, it gets skipped → the UI keeps going 

--- 

## Requirements 

* macOS (Intel or Apple Silicon) 
* Go 1.21+ (1.22 recommended) 

--- 

## Install 

### Clone it 

```bash 
git clone https://github.com/drakeafk/macos-health.git
cd macos-health
``` 

### Build it 

```bash 
go build -o macos-health .
``` 

### Run it 

```bash 
./macos-health
``` 

Quit with `q` or `Ctrl+C` 

--- 

## What you’ll see 

### System overview 

```bash 
SYSTEM
CPU:  23%    Load: 1.2 0.9 0.8
MEM:  12.3 / 32 GB (Used: 38%, Pressure: Low (42%), Swap: 0.0 GB)
DISK: 420 / 1000 GB
NET:  en0  ↓ 2.1 MB/s  ↑ 340 KB/s
BAT:  82% (Discharging, 4:12)
UPT:  3d 14h
``` 

Additional sections are shown below the system overview - including host identity
information and separate process lists for CPU-heavy and memory-heavy workloads 

### Top processes 

```bash
TOP CPU PROCESSES
chrome          180% CPU   13.2% MEM
Xcode           120% CPU    9.8% MEM
node             60% CPU    2.1% MEM

TOP MEM PROCESSES
chrome           42% CPU   18.9% MEM
Xcode            30% CPU   14.1% MEM
Docker            5% CPU    9.7% MEM
``` 

Processes are split to make it obvious whether CPU pressure or memory pressure
is responsible for slowdowns

No graphs - No animations - Just numbers that update 

--- 

## Why this exists 

### Non-blocking by default 

I tried one terminal dashboard on macOS and it froze almost immediately 

All metric collection runs asynchronously with timeouts 
If a syscall hangs, it gets dropped and the UI keeps updating 

--- 

### CPU usage that doesn’t lie 

CPU usage is computed from deltas of `cpu.Times` 
There are no sleeps and no blocking calls just to calculate percentages 

First sample is zero - That’s expected - Then it stabilizes 

--- 

### macOS memory pressure (the important part)

macOS memory usage percentages are misleading.

This tool estimates memory pressure using:

* `vm_stat`
* page size
* total system memory

It categorizes pressure as:

* Low (< 75% used)
* Medium (75–90%)
* High (> 90%)

This isn’t Apple’s internal algorithm - but it correlates well with
“is my system about to feel bad”

--- 

### Network stats that make sense

You don’t need stats for every interface.

This tool:

1. Prefers `en0` when it is active
2. Falls back to the first active non-loopback interface

Rates are calculated from byte deltas between refreshes.

--- 

## Project layout

```bash
.
├── main.go
├── go.mod
├── go.sum
└── internal
    ├── ui
    │   ├── styles.go
    │   └── view.go
    └── stats
        ├── stats.go
        ├── cpu_nonblocking.go
        ├── loadavg_darwin.go
        ├── mem_pressure_darwin.go
        ├── net_active_darwin.go
        ├── battery_darwin.go
        ├── hostinfo_darwin.go
        └── top_procs.go
``` 

* `internal/ui` handles rendering and layout 
* `internal/stats` does macOS-specific system data collection 
* `internal/` is intentional — this is an application not a library 

--- 

## Development 

Normal Go workflow 

```bash 
go fmt ./...
go vet ./...
go build ./...
go run .
``` 

Dependencies are managed with Go modules 

```bash 
go mod tidy
``` 

--- 

## Known limitations 

* Some system identity details are best-effort due to macOS privacy restrictions
* No temperature or fan data (macOS makes this painful - *at least when i tried*) 
* Process CPU percentages are best-effort 
* Disk usage only reports the root volume 
* No configuration file (yet) 

--- 

## Roadmap (maybe) 

Things I might add but no promises: 

* Better per-process CPU smoothing 
* Optional temperature/fan data (best-effort) 
* Configurable refresh interval 
* Multi-volume disk stats 
* JSON snapshot output for scripting 


--- 

## Versioning 

This project uses a simple SemVer-inspired approach 

* `v0.x` — things may change 
* `v1.0.0` — stable interface (eventually) 

See `CHANGELOG.md` for details 

--- 

## License 

MIT 

Do whatever you want with it - Just don’t blame me if it breaks 

