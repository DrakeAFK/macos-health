//go:build darwin

package stats

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	gnet "github.com/shirou/gopsutil/v3/net"
)

type netTotals struct {
	sent uint64
	recv uint64
	at   time.Time
}

type netRate struct {
	name      string
	down      float64
	up        float64
	available bool
}

func (c *Collector) collectNetwork(ctx context.Context, now time.Time) NetworkStats {
	result := NetworkStats{Status: MetricStatus{SampledAt: now}}
	interfaces, ifaceErr := gnet.InterfacesWithContext(ctx)
	counters, countersErr := nativeNetCounters(ctx)
	if ifaceErr != nil || countersErr != nil {
		result.Status = unavailableStatus(now, joinErrors(ifaceErr, countersErr))
		return result
	}

	up := make(map[string]gnet.InterfaceStat)
	for _, iface := range interfaces {
		if interfaceUsable(iface) {
			up[iface.Name] = iface
			for _, raw := range iface.Addrs {
				ip := parseIP(raw.Addr)
				if ip == nil || !usableIP(ip) {
					continue
				}
				result.Addresses = append(result.Addresses, IPAddr{Interface: cleanText(iface.Name), Address: cleanText(ip.String())})
			}
		}
	}
	sort.Slice(result.Addresses, func(i, j int) bool {
		if result.Addresses[i].Interface == result.Addresses[j].Interface {
			return result.Addresses[i].Address < result.Addresses[j].Address
		}
		return result.Addresses[i].Interface < result.Addresses[j].Interface
	})

	defaultIface := c.routeInterface(ctx, now)
	rates := make([]netRate, 0, len(counters))

	c.netMu.Lock()
	for _, row := range counters {
		if _, ok := up[row.Name]; !ok {
			continue
		}
		current := netTotals{sent: row.BytesSent, recv: row.BytesRecv, at: now}
		previous, exists := c.lastNet[row.Name]
		c.lastNet[row.Name] = current
		rate := netRate{name: row.Name}
		if exists {
			dt := now.Sub(previous.at).Seconds()
			if dt > 0 && dt < 30 && row.BytesRecv >= previous.recv && row.BytesSent >= previous.sent {
				rate.down = float64(row.BytesRecv-previous.recv) / dt
				rate.up = float64(row.BytesSent-previous.sent) / dt
				rate.available = true
			}
		}
		rates = append(rates, rate)
	}
	c.netMu.Unlock()

	if len(rates) == 0 {
		result.Status = unavailableStatus(now, fmt.Errorf("no active network interface"))
		return result
	}

	chosen := rates[0]
	for _, rate := range rates {
		if rate.available && (!chosen.available || rate.down+rate.up > chosen.down+chosen.up) {
			chosen = rate
		}
	}
	if chosen.down+chosen.up == 0 && defaultIface != "" {
		for _, rate := range rates {
			if rate.name == defaultIface && (rate.available || !chosen.available) {
				chosen = rate
				break
			}
		}
	}

	result.Status = availableStatus(now)
	result.Interface = cleanText(chosen.name)
	result.DownBytesPerSecond = chosen.down
	result.UpBytesPerSecond = chosen.up
	result.RatesAvailable = chosen.available
	return result
}

func (c *Collector) routeInterface(ctx context.Context, now time.Time) string {
	c.netMu.Lock()
	if !c.defaultIfaceAt.IsZero() && now.Sub(c.defaultIfaceAt) < 10*time.Second {
		value := c.defaultIface
		c.netMu.Unlock()
		return value
	}
	c.netMu.Unlock()

	out, err := commandOutput(ctx, "/sbin/route", "-n", "get", "default")
	if err != nil {
		c.netMu.Lock()
		c.defaultIface = ""
		c.defaultIfaceAt = now
		c.netMu.Unlock()
		return ""
	}
	value := parseRouteInterface(string(out))
	c.netMu.Lock()
	c.defaultIface = value
	c.defaultIfaceAt = now
	c.netMu.Unlock()
	return value
}

func parseRouteInterface(output string) string {
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(key) == "interface" {
			return cleanText(value)
		}
	}
	return ""
}

func interfaceUsable(iface gnet.InterfaceStat) bool {
	up, loopback := false, false
	for _, flag := range iface.Flags {
		switch flag {
		case "up":
			up = true
		case "loopback":
			loopback = true
		}
	}
	if !up || loopback {
		return false
	}
	for _, address := range iface.Addrs {
		if usableIP(parseIP(address.Addr)) {
			return true
		}
	}
	return false
}

func parseIP(value string) net.IP {
	if strings.Contains(value, "/") {
		ip, _, err := net.ParseCIDR(value)
		if err == nil {
			return ip
		}
		return nil
	}
	return net.ParseIP(value)
}

func usableIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return false
	}
	return true
}
