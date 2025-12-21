//go:build darwin

package stats

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	gnet "github.com/shirou/gopsutil/v3/net"
)

type IPAddr struct {
	Iface string
	Addr  string
}

type HostInfo struct {
	ComputerName string
	Hostname     string

	OSVersion string
	OSBuild   string
	Arch      string

	CPUModel string
	CPUCores int

	GPUModel    string
	PowerSource string

	WifiSSID string

	IPs []IPAddr
}

var hostMu sync.Mutex
var hostCached HostInfo
var hostCachedAt time.Time

const hostInfoTTL = 15 * time.Second

func getHostInfo(ctx context.Context) HostInfo {
	hostMu.Lock()
	defer hostMu.Unlock()

	if !hostCachedAt.IsZero() && time.Since(hostCachedAt) < hostInfoTTL {
		return hostCached
	}

	hi := HostInfo{
		Arch: runtime.GOARCH,
	}

	if hn, err := os.Hostname(); err == nil {
		hi.Hostname = hn
	}

	hi.ComputerName = scutilGet(ctx, "ComputerName")
	hi.OSVersion = swVers(ctx, "-productVersion")
	hi.OSBuild = swVers(ctx, "-buildVersion")

	hi.CPUCores = int(sysctlUintOr(ctx, "hw.ncpu", uint64(runtime.NumCPU())))
	hi.CPUModel = sysctlString(ctx, "machdep.cpu.brand_string")
	if strings.TrimSpace(hi.CPUModel) == "" {
		hi.CPUModel = sysctlString(ctx, "hw.model")
	}

	hi.GPUModel = gpuModel(ctx)
	hi.PowerSource = powerSource(ctx)
	hi.WifiSSID = wifiSSID(ctx)
	hi.IPs = localIPs()

	hostCached = hi
	hostCachedAt = time.Now()
	return hi
}

func scutilGet(ctx context.Context, key string) string {
	out, err := exec.CommandContext(ctx, "scutil", "--get", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func swVers(ctx context.Context, arg string) string {
	out, err := exec.CommandContext(ctx, "sw_vers", arg).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func sysctlString(ctx context.Context, key string) string {
	out, err := exec.CommandContext(ctx, "sysctl", "-n", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func sysctlUintOr(ctx context.Context, key string, def uint64) uint64 {
	out, err := exec.CommandContext(ctx, "sysctl", "-n", key).Output()
	if err != nil {
		return def
	}
	s := strings.TrimSpace(string(out))
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return def
	}
	return v
}

func gpuModel(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "system_profiler", "SPDisplaysDataType", "-detailLevel", "mini")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "Chipset Model:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Chipset Model:"))
		}
	}
	return ""
}

func powerSource(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "pmset", "-g", "batt").Output()
	if err != nil {
		return "n/a"
	}
	text := strings.ToLower(string(out))
	if strings.Contains(text, "ac power") {
		return "AC Power"
	}
	if strings.Contains(text, "battery power") {
		return "Battery Power"
	}
	return "n/a"
}

func wifiSSID(ctx context.Context) string {
	dev := wifiDevice(ctx)
	if dev == "" {
		return ""
	}
	out, err := exec.CommandContext(ctx, "networksetup", "-getairportnetwork", dev).Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	if !strings.Contains(s, ":") {
		return ""
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	ssid := strings.TrimSpace(parts[1])
	if strings.EqualFold(ssid, "off") || strings.Contains(strings.ToLower(ssid), "not associated") {
		return ""
	}
	return ssid
}

func wifiDevice(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "networksetup", "-listallhardwareports").Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")

	isWiFiBlock := false
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "Hardware Port:") {
			val := strings.TrimSpace(strings.TrimPrefix(l, "Hardware Port:"))
			isWiFiBlock = strings.EqualFold(val, "Wi-Fi") || strings.EqualFold(val, "AirPort")
			continue
		}
		if isWiFiBlock && strings.HasPrefix(l, "Device:") {
			return strings.TrimSpace(strings.TrimPrefix(l, "Device:"))
		}
	}
	return ""
}

func localIPs() []IPAddr {
	ifaces, err := gnet.Interfaces()
	if err != nil {
		return localIPsStdlib()
	}

	var res []IPAddr
	for _, itf := range ifaces {
		if !ifaceUpNonLoopback(itf) {
			continue
		}
		for _, a := range itf.Addrs {
			ip := parseIPFromCIDR(a.Addr)
			if ip == nil || !isUsableLocalIP(ip) {
				continue
			}
			res = append(res, IPAddr{Iface: itf.Name, Addr: ip.String()})
		}
	}

	sort.Slice(res, func(i, j int) bool {
		if res[i].Iface == res[j].Iface {
			return res[i].Addr < res[j].Addr
		}
		return res[i].Iface < res[j].Iface
	})

	return dedupeIPAddrs(res)
}

func localIPsStdlib() []IPAddr {
	nifs, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var res []IPAddr
	for _, nif := range nifs {
		if nif.Flags&net.FlagUp == 0 || nif.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := nif.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ip := parseIPFromCIDR(a.String())
			if ip == nil || !isUsableLocalIP(ip) {
				continue
			}
			res = append(res, IPAddr{Iface: nif.Name, Addr: ip.String()})
		}
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].Iface == res[j].Iface {
			return res[i].Addr < res[j].Addr
		}
		return res[i].Iface < res[j].Iface
	})
	return dedupeIPAddrs(res)
}

func ifaceUpNonLoopback(itf gnet.InterfaceStat) bool {
	up := false
	loop := false

	for _, f := range itf.Flags {
		switch f {
		case "up":
			up = true
		case "loopback":
			loop = true
		}
	}
	if !up || loop {
		return false
	}
	return len(itf.Addrs) > 0
}

func parseIPFromCIDR(s string) net.IP {
	if strings.Contains(s, "/") {
		ip, _, err := net.ParseCIDR(s)
		if err != nil {
			return nil
		}
		return ip
	}
	return net.ParseIP(s)
}

func isUsableLocalIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() {
		return false
	}

	if ip.To4() != nil {
		if strings.HasPrefix(ip.String(), "169.254.") {
			return false
		}
		return true
	}

	s := strings.ToLower(ip.String())
	if strings.HasPrefix(s, "fe80:") {
		return false
	}
	return true
}

func dedupeIPAddrs(in []IPAddr) []IPAddr {
	seen := map[string]bool{}
	out := make([]IPAddr, 0, len(in))
	for _, v := range in {
		k := v.Iface + "|" + v.Addr
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	return out
}
