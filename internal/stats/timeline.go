package stats

import "time"

// Trend keeps only metric data. Full process tables and host addresses must not
// be duplicated at every sample in long-running sessions.
func Trend(s Snapshot) Snapshot {
	s.Services = ServicesStats{}
	s.Processes = ProcessStats{}
	s.Host = HostInfo{}
	s.Network.Addresses = nil
	s.CPU.PerCorePercent = nil
	s.Health = Health{}
	s.Silicon.FanRPM = nil
	return s
}

type Event struct {
	At       time.Time `json:"at"`
	Severity Severity  `json:"severity"`
	Code     string    `json:"code"`
	Title    string    `json:"title"`
	Detail   string    `json:"detail"`
	Resolved bool      `json:"resolved"`
}

// Events records transitions, not one duplicate warning per refresh.
func Events(previous, current Snapshot) []Event {
	old := make(map[string]Issue)
	next := make(map[string]Issue)
	for _, i := range previous.Health.Issues {
		old[i.Code] = i
	}
	for _, i := range current.Health.Issues {
		next[i.Code] = i
	}
	events := []Event{}
	for _, i := range current.Health.Issues {
		if prior, ok := old[i.Code]; !ok || prior.Severity != i.Severity {
			events = append(events, Event{At: current.SampledAt, Severity: i.Severity, Code: i.Code, Title: i.Title, Detail: i.Detail})
		}
	}
	// Incomplete collection is not evidence that a condition resolved.
	if current.Health.Confidence == "complete" {
		for _, i := range previous.Health.Issues {
			if _, ok := next[i.Code]; !ok && issueObservable(i.Code, current) {
				events = append(events, Event{At: current.SampledAt, Severity: SeverityInfo, Code: i.Code, Title: i.Title, Resolved: true})
			}
		}
	}
	return events
}

func issueObservable(code string, s Snapshot) bool {
	fresh := func(v MetricStatus) bool { return v.Available && !v.Stale }
	switch code {
	case "memory-pressure":
		return fresh(s.Memory.Status) && s.Memory.PressureAvailable
	case "page-outs":
		return fresh(s.Memory.Status) && s.Memory.PageOutRateAvailable
	case "thermal":
		return fresh(s.Thermal.Status)
	case "battery-health", "battery-service":
		return fresh(s.Battery.Status) && s.Battery.DetailsAvailable
	case "battery-low":
		return fresh(s.Battery.Status)
	case "cpu-sustained":
		return fresh(s.CPU.Status) && !s.CPU.WarmingUp
	case "disk-full", "disk-low":
		return fresh(s.Disk.Status)
	default:
		return true
	}
}
