//go:build darwin

package stats

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (c *Collector) services(ctx context.Context, now time.Time) ServicesStats {
	if v, ok := cacheRead(&c.cacheMu, &c.servicesCache, 10*time.Second, now); ok {
		return v
	}
	out, err := commandOutput(ctx, "/usr/sbin/lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-Fpcn")
	result := ServicesStats{Enabled: true}
	// lsof returns 1 when there are no matching files, which is an empty success.
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			result.Status = unavailableStatus(now, err)
			return result
		}
	}
	listeners, parseErr := parseListeners(string(out))
	if parseErr != nil {
		result.Status = unavailableStatus(now, parseErr)
		return result
	}
	result.Listeners = listeners
	result.Status = availableStatus(now)
	cacheWrite(&c.cacheMu, &c.servicesCache, result, now)
	return result
}
func parseListeners(s string) ([]Listener, error) {
	rows := []Listener{}
	var pid int64
	var name string
	seen := map[string]bool{}
	for _, line := range strings.Split(s, "\n") {
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case 'p':
			var err error
			pid, err = strconv.ParseInt(line[1:], 10, 32)
			if err != nil || pid <= 0 {
				return nil, fmt.Errorf("invalid listener PID")
			}
			name = ""
		case 'c':
			name = cleanText(line[1:])
		case 'n':
			if pid <= 0 {
				return nil, fmt.Errorf("listener without process")
			}
			addr := cleanText(line[1:])
			key := fmt.Sprintf("%d/%s", pid, addr)
			if !seen[key] {
				rows = append(rows, Listener{PID: int32(pid), Process: name, Address: addr})
				seen[key] = true
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Address == rows[j].Address {
			return rows[i].PID < rows[j].PID
		}
		return rows[i].Address < rows[j].Address
	})
	return rows, nil
}
