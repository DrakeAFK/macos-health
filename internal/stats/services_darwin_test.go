//go:build darwin

package stats

import "testing"

func TestParseListeners(t *testing.T) {
	rows, err := parseListeners("p101\ncnode\nf12\nn127.0.0.1:3000\nf13\nn127.0.0.1:3000\np102\ncollama\nn*:11434\n")
	if err != nil || len(rows) != 2 {
		t.Fatalf("%+v %v", rows, err)
	}
	for _, bad := range []string{"pbad\nn*:8", "n*:8", "p-1\nn*:8"} {
		if _, err := parseListeners(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if rows, err := parseListeners(""); err != nil || len(rows) != 0 {
		t.Fatal("empty listeners not supported")
	}
}
func TestParseProcessesWithParent(t *testing.T) {
	rows, err := parseProcesses("101 12 01:02 00:10.50 1.5 2048 /Applications/My App.app/Contents/MacOS/My App")
	if err != nil || len(rows) != 1 || rows[0].row.ParentPID != 12 || rows[0].row.App != "My App" {
		t.Fatalf("%+v %v", rows, err)
	}
}
