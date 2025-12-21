package stats

import (
	"context"
	"time"

	gnet "github.com/shirou/gopsutil/v3/net"
)

type netTotals struct {
	sent uint64
	recv uint64
	at   time.Time
}

var lastByIface = map[string]netTotals{}

func activeNetRate(ctx context.Context) (iface string, downBps float64, upBps float64) {
	_ = ctx

	iface = pickActiveIface()
	if iface == "" {
		return "", 0, 0
	}

	io, err := gnet.IOCounters(true)
	if err != nil {
		return iface, 0, 0
	}

	var sent, recv uint64
	found := false
	for _, row := range io {
		if row.Name == iface {
			sent = row.BytesSent
			recv = row.BytesRecv
			found = true
			break
		}
	}
	if !found {
		return iface, 0, 0
	}

	now := time.Now()
	cur := netTotals{sent: sent, recv: recv, at: now}

	prev, ok := lastByIface[iface]
	if !ok {
		lastByIface[iface] = cur
		return iface, 0, 0
	}

	dt := cur.at.Sub(prev.at).Seconds()
	if dt <= 0 {
		lastByIface[iface] = cur
		return iface, 0, 0
	}

	down := float64(cur.recv-prev.recv) / dt
	up := float64(cur.sent-prev.sent) / dt

	lastByIface[iface] = cur
	return iface, down, up
}

func pickActiveIface() string {
	ifaces, err := gnet.Interfaces()
	if err != nil {
		return ""
	}

	for _, itf := range ifaces {
		if itf.Name == "en0" && isUpNonLoopback(itf) {
			return itf.Name
		}
	}

	for _, itf := range ifaces {
		if isUpNonLoopback(itf) {
			return itf.Name
		}
	}

	return ""
}

func isUpNonLoopback(itf gnet.InterfaceStat) bool {
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
