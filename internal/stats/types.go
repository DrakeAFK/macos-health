package stats

import "time"

const SnapshotSchemaVersion = 2

// MetricStatus makes missing and stale telemetry explicit. A zero value is not
// assumed to be a successful reading.
type MetricStatus struct {
	Available bool      `json:"available"`
	Stale     bool      `json:"stale,omitempty"`
	SampledAt time.Time `json:"sampled_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type CPUStats struct {
	Status         MetricStatus `json:"status"`
	UsagePercent   float64      `json:"usage_percent"`
	PerCorePercent []float64    `json:"per_core_percent,omitempty"`
	Load1          float64      `json:"load_1"`
	Load5          float64      `json:"load_5"`
	Load15         float64      `json:"load_15"`
	LoadAvailable  bool         `json:"load_available"`
	WarmingUp      bool         `json:"warming_up,omitempty"`
}

type MemoryStats struct {
	Status                MetricStatus `json:"status"`
	UsedBytes             uint64       `json:"used_bytes"`
	TotalBytes            uint64       `json:"total_bytes"`
	UsedPercent           float64      `json:"used_percent"`
	AvailablePercent      float64      `json:"available_percent"`
	Pressure              string       `json:"pressure"`
	PressureAvailable     bool         `json:"pressure_available"`
	CompressedBytes       uint64       `json:"compressed_bytes"`
	CompressionAvailable  bool         `json:"compression_available"`
	SwapUsedBytes         uint64       `json:"swap_used_bytes"`
	PageOutBytesPerSecond float64      `json:"page_out_bytes_per_second"`
	PageOutRateAvailable  bool         `json:"page_out_rate_available"`
}

type DiskStats struct {
	Status              MetricStatus `json:"status"`
	Mount               string       `json:"mount"`
	Device              string       `json:"device,omitempty"`
	UsedBytes           uint64       `json:"used_bytes"`
	FreeBytes           uint64       `json:"free_bytes"`
	TotalBytes          uint64       `json:"total_bytes"`
	UsedPercent         float64      `json:"used_percent"`
	ReadBytesPerSecond  float64      `json:"read_bytes_per_second"`
	WriteBytesPerSecond float64      `json:"write_bytes_per_second"`
	IOAvailable         bool         `json:"io_available"`
}

type IPAddr struct {
	Interface string `json:"interface"`
	Address   string `json:"address"`
}

type NetworkStats struct {
	Status             MetricStatus `json:"status"`
	Interface          string       `json:"interface"`
	DownBytesPerSecond float64      `json:"down_bytes_per_second"`
	UpBytesPerSecond   float64      `json:"up_bytes_per_second"`
	RatesAvailable     bool         `json:"rates_available"`
	Addresses          []IPAddr     `json:"addresses,omitempty"`
}

type BatteryStats struct {
	Status                 MetricStatus `json:"status"`
	Present                bool         `json:"present"`
	Percent                float64      `json:"percent"`
	State                  string       `json:"state"`
	PowerSource            string       `json:"power_source"`
	TimeRemainingMinutes   int          `json:"time_remaining_minutes,omitempty"`
	TimeRemainingAvailable bool         `json:"time_remaining_available"`
	HealthPercent          float64      `json:"health_percent,omitempty"`
	Condition              string       `json:"condition,omitempty"`
	CycleCount             int          `json:"cycle_count,omitempty"`
	TemperatureC           float64      `json:"temperature_c,omitempty"`
	TemperatureAvailable   bool         `json:"temperature_available"`
	LowPowerMode           bool         `json:"low_power_mode"`
	DetailsAvailable       bool         `json:"details_available"`
	ChargerWatts           int          `json:"charger_watts,omitempty"`
}

type ThermalStats struct {
	Status                MetricStatus `json:"status"`
	State                 string       `json:"state"`
	CPUSpeedLimitPercent  int          `json:"cpu_speed_limit_percent,omitempty"`
	SchedulerLimitPercent int          `json:"scheduler_limit_percent,omitempty"`
	ThermalLevel          int          `json:"thermal_level,omitempty"`
}

type ProcessRow struct {
	MemoryAvailable     bool      `json:"memory_available"`
	StartedAt           time.Time `json:"started_at,omitempty"`
	FootprintBytes      uint64    `json:"footprint_bytes"`
	FootprintAvailable  bool      `json:"footprint_available"`
	ReadBytesPerSecond  float64   `json:"read_bytes_per_second"`
	WriteBytesPerSecond float64   `json:"write_bytes_per_second"`
	IOAvailable         bool      `json:"io_available"`
	ParentPID           int32     `json:"parent_pid,omitempty"`
	App                 string    `json:"app,omitempty"`
	Kind                string    `json:"kind,omitempty"`
	AgeSeconds          float64   `json:"age_seconds"`
	CPUAvailable        bool      `json:"cpu_available"`
	PID                 int32     `json:"pid"`
	Name                string    `json:"name"`
	CPUPercent          float64   `json:"cpu_percent"`
	MemoryPercent       float64   `json:"memory_percent"`
	RSSBytes            uint64    `json:"rss_bytes"`
}

type ProcessStats struct {
	Source    string       `json:"source"`
	All       []ProcessRow `json:"all,omitempty"`
	Total     int          `json:"total"`
	Status    MetricStatus `json:"status"`
	WarmingUp bool         `json:"warming_up,omitempty"`
	TopCPU    []ProcessRow `json:"top_cpu"`
	TopMemory []ProcessRow `json:"top_memory"`
}

type HostInfo struct {
	Status              MetricStatus `json:"status"`
	ComputerName        string       `json:"computer_name"`
	Hostname            string       `json:"hostname"`
	MachineName         string       `json:"machine_name"`
	ModelIdentifier     string       `json:"model_identifier"`
	OSVersion           string       `json:"os_version"`
	OSBuild             string       `json:"os_build"`
	Architecture        string       `json:"architecture"`
	Chip                string       `json:"chip"`
	LogicalCores        int          `json:"logical_cores"`
	PerformanceCores    int          `json:"performance_cores,omitempty"`
	EfficiencyCores     int          `json:"efficiency_cores,omitempty"`
	GPU                 string       `json:"gpu"`
	GPUCores            int          `json:"gpu_cores,omitempty"`
	NeuralEngine        bool         `json:"neural_engine,omitempty"`
	RunningUnderRosetta bool         `json:"running_under_rosetta,omitempty"`
}

type SystemStats struct {
	Status        MetricStatus `json:"status"`
	UptimeSeconds uint64       `json:"uptime_seconds"`
}

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Issue struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Title    string   `json:"title"`
	Detail   string   `json:"detail"`
}

type Health struct {
	Confidence string  `json:"confidence"`
	Status     string  `json:"status"`
	Summary    string  `json:"summary"`
	Issues     []Issue `json:"issues"`
}

type SensorValue struct {
	Available bool    `json:"available"`
	Value     float64 `json:"value"`
}

type SiliconStats struct {
	Status         MetricStatus `json:"status"`
	Enabled        bool         `json:"enabled"`
	Source         string       `json:"source"`
	CPUWatts       SensorValue  `json:"cpu_watts"`
	GPUWatts       SensorValue  `json:"gpu_watts"`
	ANEWatts       SensorValue  `json:"ane_watts"`
	GPUPercent     SensorValue  `json:"gpu_percent"`
	CPUTemperature SensorValue  `json:"cpu_temperature_c"`
	GPUTemperature SensorValue  `json:"gpu_temperature_c"`
	FanRPM         []float64    `json:"fan_rpm,omitempty"`
}

type Listener struct {
	PID     int32  `json:"pid"`
	Process string `json:"process"`
	Address string `json:"address"`
}
type ServicesStats struct {
	Status    MetricStatus `json:"status"`
	Enabled   bool         `json:"enabled"`
	Listeners []Listener   `json:"listeners"`
}
type Snapshot struct {
	Services             ServicesStats `json:"services"`
	Silicon              SiliconStats  `json:"silicon"`
	SchemaVersion        int           `json:"schema_version"`
	SampledAt            time.Time     `json:"sampled_at"`
	CollectionDurationMS int64         `json:"collection_duration_ms"`
	CPU                  CPUStats      `json:"cpu"`
	Memory               MemoryStats   `json:"memory"`
	Disk                 DiskStats     `json:"disk"`
	Network              NetworkStats  `json:"network"`
	Battery              BatteryStats  `json:"battery"`
	Thermal              ThermalStats  `json:"thermal"`
	Processes            ProcessStats  `json:"processes"`
	Host                 HostInfo      `json:"host"`
	System               SystemStats   `json:"system"`
	Health               Health        `json:"health"`
}
