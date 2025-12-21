package stats

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func battery(ctx context.Context) (percent float64, state string, eta string) {
	out, err := exec.CommandContext(ctx, "pmset", "-g", "batt").Output()
	if err != nil {
		return 0, "", ""
	}
	text := string(out)
	lower := strings.ToLower(text)

	rePct := regexp.MustCompile(`(\d+)%`)
	m := rePct.FindStringSubmatch(text)
	if len(m) >= 2 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			percent = v
		}
	}

	switch {
	case strings.Contains(lower, "not charging"):
		state = "Not Charging"
	case strings.Contains(lower, "discharging"):
		state = "Discharging"
	case strings.Contains(lower, "charging"):
		state = "Charging"
	case strings.Contains(lower, "charged"):
		state = "Charged"
	}

	reEta := regexp.MustCompile(`(\d+:\d+)\s+remaining`)
	e := reEta.FindStringSubmatch(text)
	if len(e) >= 2 {
		eta = e[1]
	}

	return percent, state, eta
}
