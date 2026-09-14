package stats

import (
	"strings"
	"time"
	"unicode"
)

func availableStatus(at time.Time) MetricStatus {
	return MetricStatus{Available: true, SampledAt: at}
}

func unavailableStatus(at time.Time, err error) MetricStatus {
	status := MetricStatus{SampledAt: at}
	if err != nil {
		status.Error = cleanText(err.Error())
	}
	return status
}

// MergeLastGood carries forward a prior successful metric when the latest
// attempt failed. The retained value is explicitly marked stale.
func MergeLastGood(current, previous Snapshot) Snapshot {
	carry := func(now, old MetricStatus) bool {
		return !now.Available && old.Available
	}

	if carry(current.CPU.Status, previous.CPU.Status) {
		err := current.CPU.Status.Error
		current.CPU = previous.CPU
		current.CPU.Status.Stale = true
		current.CPU.Status.Error = err
	}
	if carry(current.Memory.Status, previous.Memory.Status) {
		err := current.Memory.Status.Error
		current.Memory = previous.Memory
		current.Memory.Status.Stale = true
		current.Memory.Status.Error = err
	}
	if carry(current.Disk.Status, previous.Disk.Status) {
		err := current.Disk.Status.Error
		current.Disk = previous.Disk
		current.Disk.Status.Stale = true
		current.Disk.Status.Error = err
	}
	if carry(current.Network.Status, previous.Network.Status) {
		err := current.Network.Status.Error
		current.Network = previous.Network
		current.Network.Status.Stale = true
		current.Network.Status.Error = err
	}
	if carry(current.Battery.Status, previous.Battery.Status) {
		err := current.Battery.Status.Error
		current.Battery = previous.Battery
		current.Battery.Status.Stale = true
		current.Battery.Status.Error = err
	}
	if carry(current.Thermal.Status, previous.Thermal.Status) {
		err := current.Thermal.Status.Error
		current.Thermal = previous.Thermal
		current.Thermal.Status.Stale = true
		current.Thermal.Status.Error = err
	}
	if carry(current.Processes.Status, previous.Processes.Status) {
		err := current.Processes.Status.Error
		current.Processes = previous.Processes
		current.Processes.Status.Stale = true
		current.Processes.Status.Error = err
	}
	if carry(current.Host.Status, previous.Host.Status) {
		err := current.Host.Status.Error
		current.Host = previous.Host
		current.Host.Status.Stale = true
		current.Host.Status.Error = err
	}
	if carry(current.System.Status, previous.System.Status) {
		err := current.System.Status.Error
		current.System = previous.System
		current.System.Status.Stale = true
		current.System.Status.Error = err
	}

	if carry(current.Silicon.Status, previous.Silicon.Status) && current.Silicon.Enabled {
		err := current.Silicon.Status.Error
		current.Silicon = previous.Silicon
		current.Silicon.Status.Stale = true
		current.Silicon.Status.Error = err
	}
	if carry(current.Services.Status, previous.Services.Status) && current.Services.Enabled {
		err := current.Services.Status.Error
		current.Services = previous.Services
		current.Services.Status.Stale = true
		current.Services.Status.Error = err
	}
	current.Health = Assess(current, nil)
	return current
}

// Redacted returns a copy safe to share in screenshots or diagnostic output.
func (s Snapshot) Redacted() Snapshot {
	s.Host.ComputerName = "redacted"
	s.Host.Hostname = "redacted"
	s.Network.Addresses = append([]IPAddr(nil), s.Network.Addresses...)
	for i := range s.Network.Addresses {
		s.Network.Addresses[i].Address = "redacted"
	}
	s.Services.Listeners = append([]Listener(nil), s.Services.Listeners...)
	for i := range s.Services.Listeners {
		s.Services.Listeners[i].Address = "redacted"
	}
	return s
}

func cleanText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	needsClean := false
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b < 0x20 || b == 0x7f || b > 0x7e {
			needsClean = true
			break
		}
	}
	if !needsClean {
		return s
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || (r >= 0x7f && r <= 0x9f) || isBidiControl(r) {
			return -1
		}
		return r
	}, s)
}

func isBidiControl(r rune) bool {
	return r == '\u061c' || r == '\u200e' || r == '\u200f' ||
		(r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069')
}
