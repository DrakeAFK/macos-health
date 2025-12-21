# Changelog 

This file documents notable changes to `macos-health` 

Versioning is loosely inspired by Semantic Versioning
([https://semver.org](https://semver.org)), but without pretending everything is stable yet 

--- 

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

--- 

If you want, the *next* good cleanup step after this is: 

* trimming the changelog header even more, or 
* adding a `v0.1.1` entry once you make your first small fix or polish pass 

But honestly — this is already very solid and very “real project” looking 

