//go:build darwin

package stats

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
)

const (
	hostTTL           = 24 * time.Hour
	processTTL        = 2 * time.Second
	batteryStatusTTL  = 5 * time.Second
	batteryDetailsTTL = 30 * time.Second
	thermalTTL        = 5 * time.Second
	diskTTL           = 10 * time.Second
)

type timedCache[T any] struct {
	value T
	at    time.Time
	valid bool
}

type diskTotals struct {
	read  uint64
	write uint64
	at    time.Time
}

// Collector owns all delta state and cadence caches. A Collector is safe for
// concurrent callers, though the application intentionally keeps one sample in
// flight so results cannot arrive out of order.
type Collector struct {
	systemProcesses     bool
	portsEnabled        bool
	servicesCache       timedCache[ServicesStats]
	sensors             sensorReader
	sensorsEnabled      bool
	cacheMu             sync.Mutex
	hostCache           timedCache[HostInfo]
	processCache        timedCache[ProcessStats]
	batteryStatusCache  timedCache[BatteryStats]
	batteryDetailsCache timedCache[batteryDetails]
	thermalCache        timedCache[ThermalStats]
	diskCache           timedCache[DiskStats]

	cpuMu   sync.Mutex
	lastCPU map[string]cpu.TimesStat

	netMu          sync.Mutex
	lastNet        map[string]netTotals
	defaultIface   string
	defaultIfaceAt time.Time

	processMu     sync.Mutex
	lastProcesses map[int32]processSample

	memoryMu     sync.Mutex
	lastPageOuts uint64
	lastMemoryAt time.Time

	diskMu     sync.Mutex
	lastDiskIO diskTotals
}

type CollectorOptions struct {
	SystemProcesses bool
	Sensors         bool
	Ports           bool
}

func NewCollector() *Collector { return NewCollectorWithOptions(CollectorOptions{}) }

func NewCollectorWithOptions(options CollectorOptions) *Collector {
	return &Collector{
		sensorsEnabled:  options.Sensors,
		systemProcesses: options.SystemProcesses,
		portsEnabled:    options.Ports,
		lastCPU:         make(map[string]cpu.TimesStat),
		lastNet:         make(map[string]netTotals),
		lastProcesses:   make(map[int32]processSample),
	}
}

func (c *Collector) Collect(ctx context.Context) (Snapshot, error) {
	started := time.Now()
	snapshot := Snapshot{SchemaVersion: SnapshotSchemaVersion}
	snapshot.SampledAt = started

	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}

	if c.portsEnabled {
		run(func() { snapshot.Services = c.services(ctx, started) })
	}
	if c.sensorsEnabled {
		run(func() { snapshot.Silicon = c.sensors.sample(ctx, started) })
	}
	run(func() { snapshot.CPU = c.collectCPU(ctx, started) })
	run(func() { snapshot.Memory = c.collectMemory(ctx, started) })
	run(func() { snapshot.Disk = c.collectDisk(ctx, started) })
	run(func() { snapshot.Network = c.collectNetwork(ctx, started) })
	run(func() { snapshot.Battery = c.battery(ctx, started) })
	run(func() { snapshot.Thermal = c.thermal(ctx, started) })
	run(func() { snapshot.Processes = c.processes(ctx, started) })
	run(func() { snapshot.Host = c.host(ctx, started) })
	run(func() {
		uptime, err := host.UptimeWithContext(ctx)
		snapshot.System.Status = unavailableStatus(started, err)
		if err == nil {
			snapshot.System.Status = availableStatus(started)
			snapshot.System.UptimeSeconds = uptime
		}
	})

	wg.Wait()
	snapshot.CollectionDurationMS = time.Since(started).Milliseconds()
	snapshot.Health = Assess(snapshot, nil)

	if err := ctx.Err(); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func (c *Collector) host(ctx context.Context, now time.Time) HostInfo {
	if cached, ok := cacheRead(&c.cacheMu, &c.hostCache, hostTTL, now); ok {
		return cached
	}

	value, err := collectHostInfo(ctx)
	if err == nil || value.Chip != "" || value.OSVersion != "" {
		value.Status = availableStatus(now)
		if err != nil {
			value.Status.Error = cleanText(err.Error())
		}
		cacheWrite(&c.cacheMu, &c.hostCache, value, now)
		return value
	}
	if prior, ok := cacheAny(&c.cacheMu, &c.hostCache); ok {
		prior.Status.Stale = true
		prior.Status.Error = cleanText(err.Error())
		return prior
	}
	value.Status = unavailableStatus(now, err)
	return value
}

func (c *Collector) diskCapacity(ctx context.Context, now time.Time) DiskStats {
	if cached, ok := cacheRead(&c.cacheMu, &c.diskCache, diskTTL, now); ok {
		return cached
	}

	const mount = "/System/Volumes/Data"
	actualMount := mount
	usage, err := disk.UsageWithContext(ctx, mount)
	if err != nil {
		actualMount = "/"
		usage, err = disk.UsageWithContext(ctx, "/")
	}
	if err != nil {
		if prior, ok := cacheAny(&c.cacheMu, &c.diskCache); ok {
			prior.Status.Stale = true
			prior.Status.Error = cleanText(err.Error())
			return prior
		}
		return DiskStats{Status: unavailableStatus(now, err)}
	}

	// APFS volumes share their container. Total - free reflects all sibling
	// volumes and snapshots; Usage.Used for the sealed root or Data volume alone
	// substantially undercounts container consumption.
	used := uint64(0)
	if usage.Total >= usage.Free {
		used = usage.Total - usage.Free
	}
	percent := float64(0)
	if usage.Total > 0 {
		percent = float64(used) / float64(usage.Total) * 100
	}
	value := DiskStats{
		Status:      availableStatus(now),
		Mount:       actualMount,
		UsedBytes:   used,
		FreeBytes:   usage.Free,
		TotalBytes:  usage.Total,
		UsedPercent: percent,
	}
	cacheWrite(&c.cacheMu, &c.diskCache, value, now)
	return value
}

func (c *Collector) collectDisk(ctx context.Context, now time.Time) DiskStats {
	result := c.diskCapacity(ctx, now)
	if !result.Status.Available {
		return result
	}
	if !result.Status.Stale {
		result.Status.SampledAt = now
	}

	counters, err := disk.IOCountersWithContext(ctx, "disk0")
	row, ok := counters["disk0"]
	if err != nil || !ok {
		if err != nil {
			result.Status.Error = cleanText(err.Error())
		}
		return result
	}
	result.Device = "disk0"

	c.diskMu.Lock()
	previous := c.lastDiskIO
	current := diskTotals{read: row.ReadBytes, write: row.WriteBytes, at: now}
	c.lastDiskIO = current
	result.ReadBytesPerSecond, result.WriteBytesPerSecond, result.IOAvailable = diskIORates(previous, current)
	c.diskMu.Unlock()
	return result
}

func diskIORates(previous, current diskTotals) (read, write float64, ok bool) {
	if previous.at.IsZero() || current.read < previous.read || current.write < previous.write {
		return 0, 0, false
	}
	dt := current.at.Sub(previous.at).Seconds()
	if dt <= 0 || dt >= 30 {
		return 0, 0, false
	}
	return float64(current.read-previous.read) / dt, float64(current.write-previous.write) / dt, true
}

func (c *Collector) processes(ctx context.Context, now time.Time) ProcessStats {
	if cached, ok := cacheRead(&c.cacheMu, &c.processCache, processTTL, now); ok && !cached.WarmingUp {
		return cached
	}
	value, err := c.collectProcesses(ctx, now, 12)
	if err == nil {
		value.Status = availableStatus(now)
		cacheWrite(&c.cacheMu, &c.processCache, value, now)
		return value
	}
	if prior, ok := cacheAny(&c.cacheMu, &c.processCache); ok {
		prior.Status.Stale = true
		prior.Status.Error = cleanText(err.Error())
		return prior
	}
	value.Status = unavailableStatus(now, err)
	return value
}

func (c *Collector) thermal(ctx context.Context, now time.Time) ThermalStats {
	if cached, ok := cacheRead(&c.cacheMu, &c.thermalCache, thermalTTL, now); ok {
		return cached
	}
	value, err := collectThermal(ctx)
	if err == nil {
		value.Status = availableStatus(now)
		cacheWrite(&c.cacheMu, &c.thermalCache, value, now)
		return value
	}
	if prior, ok := cacheAny(&c.cacheMu, &c.thermalCache); ok {
		prior.Status.Stale = true
		prior.Status.Error = cleanText(err.Error())
		return prior
	}
	value.Status = unavailableStatus(now, err)
	return value
}

func (c *Collector) battery(ctx context.Context, now time.Time) BatteryStats {
	if cached, ok := cacheRead(&c.cacheMu, &c.batteryStatusCache, batteryStatusTTL, now); ok {
		return cached
	}
	value := c.collectBattery(ctx, now)
	if value.Status.Available {
		cacheWrite(&c.cacheMu, &c.batteryStatusCache, value, now)
		return value
	}
	if prior, ok := cacheAny(&c.cacheMu, &c.batteryStatusCache); ok {
		prior.Status.Stale = true
		prior.Status.Error = value.Status.Error
		return prior
	}
	return value
}

func (c *Collector) batteryDetails(ctx context.Context, now time.Time) (batteryDetails, error) {
	if cached, ok := cacheRead(&c.cacheMu, &c.batteryDetailsCache, batteryDetailsTTL, now); ok {
		return cached, nil
	}
	value, err := collectBatteryDetails(ctx)
	if err == nil {
		cacheWrite(&c.cacheMu, &c.batteryDetailsCache, value, now)
		return value, nil
	}
	if prior, ok := cacheAny(&c.cacheMu, &c.batteryDetailsCache); ok {
		return prior, fmt.Errorf("battery details: %w", err)
	}
	return value, err
}

func cacheRead[T any](mu *sync.Mutex, cache *timedCache[T], ttl time.Duration, now time.Time) (T, bool) {
	mu.Lock()
	defer mu.Unlock()
	if cache.valid && now.Sub(cache.at) < ttl {
		return cache.value, true
	}
	var zero T
	return zero, false
}

func cacheAny[T any](mu *sync.Mutex, cache *timedCache[T]) (T, bool) {
	mu.Lock()
	defer mu.Unlock()
	if cache.valid {
		return cache.value, true
	}
	var zero T
	return zero, false
}

func cacheWrite[T any](mu *sync.Mutex, cache *timedCache[T], value T, now time.Time) {
	mu.Lock()
	cache.value = value
	cache.at = now
	cache.valid = true
	mu.Unlock()
}

func joinErrors(errs ...error) error {
	var nonNil []error
	for _, err := range errs {
		if err != nil {
			nonNil = append(nonNil, err)
		}
	}
	return errors.Join(nonNil...)
}

func (c *Collector) Close() { c.sensors.close() }
