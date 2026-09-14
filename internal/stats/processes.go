package stats

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Preserve the owning app without retaining its path, arguments or environment.
func processApp(command string) string {
	if i := strings.Index(command, ".app/"); i >= 0 {
		return cleanText(filepath.Base(command[:i]))
	}
	return cleanText(filepath.Base(command))
}

func processKind(name string) string {
	n := strings.ToLower(name)
	for _, word := range []string{"ollama", "llama", "lm studio", "mlx", "vllm", "localai"} {
		if strings.Contains(n, word) {
			return "AI"
		}
	}
	for _, word := range []string{"node", "python", "docker", "container", "code", "xcode", "rustc", "cargo", "clang", "swift", "go", "java", "postgres", "redis"} {
		if n == word || strings.HasPrefix(n, word+" ") || strings.Contains(n, word+" helper") {
			return "Dev"
		}
	}
	return ""
}

type ProcessGroup struct {
	Name       string  `json:"name"`
	Kind       string  `json:"kind,omitempty"`
	Count      int     `json:"count"`
	CPUPercent float64 `json:"cpu_percent"`
	RSSBytes   uint64  `json:"rss_bytes"`
}

func GroupProcesses(rows []ProcessRow) []ProcessGroup {
	groups := make(map[string]*ProcessGroup)
	for _, p := range rows {
		name := p.App
		if name == "" {
			name = p.Name
		}
		g := groups[name]
		if g == nil {
			g = &ProcessGroup{Name: name, Kind: p.Kind}
			groups[name] = g
		}
		if g.Kind == "" || p.Kind == "AI" {
			g.Kind = p.Kind
		}
		g.Count++
		if p.CPUAvailable {
			g.CPUPercent += p.CPUPercent
		}
		g.RSSBytes += p.RSSBytes
	}
	result := make([]ProcessGroup, 0, len(groups))
	for _, g := range groups {
		result = append(result, *g)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CPUPercent == result[j].CPUPercent {
			if result[i].RSSBytes == result[j].RSSBytes {
				return result[i].Name < result[j].Name
			}
			return result[i].RSSBytes > result[j].RSSBytes
		}
		return result[i].CPUPercent > result[j].CPUPercent
	})
	return result
}

// ProcessRows returns a filtered copy; sorting never mutates collector caches.
func ProcessRows(p ProcessStats, query, sortBy string) []ProcessRow {
	source := p.All
	if source == nil {
		source = p.TopCPU
		if sortBy == "memory" {
			source = p.TopMemory
		}
	}
	result := make([]ProcessRow, 0, len(source))
	query = strings.ToLower(strings.TrimSpace(query))
	for _, row := range source {
		if query == "" || strings.Contains(strings.ToLower(row.Name+" "+row.App+" "+row.Kind+" "+strconv.Itoa(int(row.PID))), query) {
			result = append(result, row)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if sortBy == "memory" && a.RSSBytes != b.RSSBytes {
			return a.RSSBytes > b.RSSBytes
		}
		if a.CPUPercent != b.CPUPercent {
			return a.CPUPercent > b.CPUPercent
		}
		if a.RSSBytes != b.RSSBytes {
			return a.RSSBytes > b.RSSBytes
		}
		return a.PID < b.PID
	})
	return result
}
