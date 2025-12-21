# Changelog 

This file documents notable changes to `macos-health` 

Versioning is loosely inspired by Semantic Versioning
([https://semver.org](https://semver.org)), but without pretending everything is stable yet 

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
