package stats

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Prometheus omits absent and stale values instead of exporting a false zero.
func Prometheus(s Snapshot) string {
	var b strings.Builder
	gauge := func(name, help string, value float64) {
		fmt.Fprintf(&b, "# HELP macos_health_%s %s\n# TYPE macos_health_%s gauge\nmacos_health_%s %g\n", name, help, name, name, value)
	}
	fresh := func(st MetricStatus) bool { return st.Available && !st.Stale }
	for _, m := range []struct {
		name   string
		status MetricStatus
	}{{"cpu", s.CPU.Status}, {"memory", s.Memory.Status}, {"disk", s.Disk.Status}, {"network", s.Network.Status}, {"battery", s.Battery.Status}, {"silicon", s.Silicon.Status}} {
		v := 0.
		if fresh(m.status) {
			v = 1
		}
		gauge(m.name+"_available", "Reading is available and fresh.", v)
	}
	gauge("sample_timestamp_seconds", "Unix time of snapshot.", float64(s.SampledAt.UnixMilli())/1000)
	gauge("collection_duration_seconds", "Wall time spent collecting this snapshot.", float64(s.CollectionDurationMS)/1000)
	if fresh(s.CPU.Status) {
		gauge("cpu_percent", "CPU use across all logical cores (0-100).", s.CPU.UsagePercent)
	}
	if fresh(s.Memory.Status) {
		gauge("memory_used_bytes", "Used physical memory.", float64(s.Memory.UsedBytes))
		gauge("memory_total_bytes", "Total physical memory.", float64(s.Memory.TotalBytes))
		gauge("swap_used_bytes", "Swap allocated; not itself evidence of pressure.", float64(s.Memory.SwapUsedBytes))
	}
	if fresh(s.Disk.Status) {
		gauge("disk_free_bytes", "Available APFS capacity.", float64(s.Disk.FreeBytes))
		if s.Disk.IOAvailable {
			gauge("disk_read_bytes_per_second", "Internal disk read rate.", s.Disk.ReadBytesPerSecond)
			gauge("disk_write_bytes_per_second", "Internal disk write rate.", s.Disk.WriteBytesPerSecond)
		}
	}
	if fresh(s.Network.Status) && s.Network.RatesAvailable {
		gauge("network_receive_bytes_per_second", "Selected interface receive rate.", s.Network.DownBytesPerSecond)
		gauge("network_transmit_bytes_per_second", "Selected interface transmit rate.", s.Network.UpBytesPerSecond)
	}
	if fresh(s.Battery.Status) && s.Battery.Present {
		gauge("battery_percent", "Battery charge (0-100).", s.Battery.Percent)
	}
	if fresh(s.Silicon.Status) {
		for _, v := range []struct {
			name   string
			sensor SensorValue
		}{{"cpu_power_watts", s.Silicon.CPUWatts}, {"gpu_power_watts", s.Silicon.GPUWatts}, {"ane_power_watts", s.Silicon.ANEWatts}, {"gpu_percent", s.Silicon.GPUPercent}, {"cpu_temperature_celsius", s.Silicon.CPUTemperature}, {"gpu_temperature_celsius", s.Silicon.GPUTemperature}} {
			if v.sensor.Available {
				gauge(v.name, "Best-effort native silicon sensor.", v.sensor.Value)
			}
		}
	}
	return b.String()
}

// JSONSchema is generated from the actual exported Go model to prevent drift.
func JSONSchema() map[string]any {
	schema := schemaFor(reflect.TypeOf(Snapshot{}))
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	schema["title"] = "macos-health snapshot v2"
	return schema
}
func schemaFor(t reflect.Type) map[string]any {
	if t == reflect.TypeOf(time.Time{}) {
		return map[string]any{"type": "string", "format": "date-time"}
	}
	switch t.Kind() {
	case reflect.Struct:
		properties := map[string]any{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")
			if tag[0] == "" || tag[0] == "-" {
				continue
			}
			properties[tag[0]] = schemaFor(f.Type)
			if len(tag) == 1 {
				required = append(required, tag[0])
			}
		}
		return map[string]any{"type": "object", "properties": properties, "required": required}
	case reflect.Slice:
		return map[string]any{"type": []string{"array", "null"}, "items": schemaFor(t.Elem())}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	default:
		return map[string]any{"type": "integer"}
	}
}
