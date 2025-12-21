package stats

import (
	"bufio"
	"context"
	"os/exec"
	"strconv"
	"strings"
)

// memPressure computes a simple "pressure" estimate from vm_stat
//
// treat "available" memory as: free + speculative + inactive
// pressure % is: used / total * 100 where used = total - available
// then map to Low/Medium/High with conservative thresholds
//
// this isn't Apple's exact memory pressure algorithm but it works
// and correlates with real-world responsiveness
func memPressure(ctx context.Context) (level string, pct float64) {
	pageSize, ok := sysctlUint(ctx, "hw.pagesize")
	if !ok || pageSize == 0 {
		return "n/a", 0
	}
	memTotal, ok := sysctlUint(ctx, "hw.memsize")
	if !ok || memTotal == 0 {
		return "n/a", 0
	}

	freePages, inactivePages, speculativePages := vmStatPages(ctx)
	if freePages < 0 {
		return "n/a", 0
	}

	availableBytes := uint64(freePages+inactivePages+speculativePages) * pageSize
	if availableBytes > memTotal {
		availableBytes = memTotal
	}

	usedBytes := memTotal - availableBytes
	pct = (float64(usedBytes) / float64(memTotal)) * 100.0

	switch {
	case pct >= 90:
		level = "High"
	case pct >= 75:
		level = "Medium"
	default:
		level = "Low"
	}

	return level, pct
}

func vmStatPages(ctx context.Context) (freePages int64, inactivePages int64, speculativePages int64) {
	cmd := exec.CommandContext(ctx, "vm_stat")
	out, err := cmd.Output()
	if err != nil {
		return -1, 0, 0
	}

	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		switch {
		case strings.HasPrefix(line, "Pages free:"):
			freePages = parseVMStatLine(line)
		case strings.HasPrefix(line, "Pages inactive:"):
			inactivePages = parseVMStatLine(line)
		case strings.HasPrefix(line, "Pages speculative:"):
			speculativePages = parseVMStatLine(line)
		}
	}
	return freePages, inactivePages, speculativePages
}

func parseVMStatLine(line string) int64 {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return 0
	}
	right := strings.TrimSpace(parts[1])
	right = strings.TrimSuffix(right, ".")
	right = strings.ReplaceAll(right, ".", "")
	right = strings.ReplaceAll(right, ",", "")
	n, _ := strconv.ParseInt(strings.Fields(right)[0], 10, 64)
	return n
}

func sysctlUint(ctx context.Context, key string) (uint64, bool) {
	out, err := exec.CommandContext(ctx, "sysctl", "-n", key).Output()
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(out))
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
