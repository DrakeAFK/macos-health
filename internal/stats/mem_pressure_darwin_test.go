//go:build darwin

package stats

import (
	"strings"
	"testing"
)

func TestParseVMCountersSupportsIntelAndAppleSiliconPageSizes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantPageSize   uint64
		wantCompressed uint64
		wantPageOuts   uint64
	}{
		{
			name: "Intel 4 KiB pages",
			input: `Mach Virtual Memory Statistics: (page size of 4096 bytes)
Pages occupied by compressor:               12,345.
Pageouts:                                     6,789.
`,
			wantPageSize: 4096, wantCompressed: 12345, wantPageOuts: 6789,
		},
		{
			name: "Apple Silicon 16 KiB pages and alternate pageout label",
			input: `Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages occupied by compressor:               601726.
Pages paged out:                             1798347.
`,
			wantPageSize: 16384, wantCompressed: 601726, wantPageOuts: 1798347,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseVMCounters(tt.input)
			if err != nil {
				t.Fatalf("parseVMCounters returned error: %v", err)
			}
			if got.PageSize != tt.wantPageSize || got.CompressedPages != tt.wantCompressed || got.PageOuts != tt.wantPageOuts {
				t.Fatalf("parseVMCounters = %+v, want pageSize=%d compressed=%d pageouts=%d", got, tt.wantPageSize, tt.wantCompressed, tt.wantPageOuts)
			}
		})
	}
}

func TestParseVMCountersRejectsMissingOrInvalidPageSize(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"",
		"Pages occupied by compressor: 12.\nPageouts: 2.\n",
		"Mach Virtual Memory Statistics: (page size of bytes)\n",
		"Mach Virtual Memory Statistics: (page size of 0 bytes)\n",
	}
	for _, input := range inputs {
		input := input
		t.Run(strings.ReplaceAll(input, "\n", "_"), func(t *testing.T) {
			t.Parallel()
			if _, err := parseVMCounters(input); err == nil {
				t.Fatalf("parseVMCounters(%q) succeeded, want page-size error", input)
			}
		})
	}
}

func TestParseVMCountersRejectsMissingCounters(t *testing.T) {
	for _, input := range []string{
		"Mach Virtual Memory Statistics: (page size of 16384 bytes)\nPageouts: 1.\n",
		"Mach Virtual Memory Statistics: (page size of 16384 bytes)\nPages occupied by compressor: 1.\n",
	} {
		if _, err := parseVMCounters(input); err == nil {
			t.Fatalf("parseVMCounters(%q) succeeded with a missing counter", input)
		}
	}
}

func TestParseVMValue(t *testing.T) {
	t.Parallel()

	valid := []struct {
		input string
		want  uint64
	}{
		{input: "0.", want: 0},
		{input: "  42.  ", want: 42},
		{input: "1,234,567.", want: 1234567},
		{input: "99 extra fields are ignored", want: 99},
	}
	for _, tt := range valid {
		got, err := parseVMValue(tt.input)
		if err != nil {
			t.Errorf("parseVMValue(%q) returned error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseVMValue(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}

	invalid := []string{"", " ", ".", "not-a-number.", "-1.", "18446744073709551616."}
	for _, input := range invalid {
		if got, err := parseVMValue(input); err == nil {
			t.Errorf("parseVMValue(%q) = %d, want error", input, got)
		}
	}
}

func TestParseVMValueEmptyNeverPanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("parseVMValue panicked on empty input: %v", recovered)
		}
	}()
	if _, err := parseVMValue(""); err == nil {
		t.Fatal("parseVMValue(\"\") succeeded, want error")
	}
}
