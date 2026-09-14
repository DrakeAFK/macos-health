//go:build darwin

package stats

import (
	"strings"
	"testing"
	"unicode"
)

func TestParseRouteInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "physical interface",
			input: `   route to: default
destination: default
       mask: default
  interface: en0
      flags: <UP,GATEWAY,DONE,STATIC,PRCLONING,GLOBAL>
`,
			want: "en0",
		},
		{name: "VPN interface", input: "interface :   utun12  \n", want: "utun12"},
		{name: "missing", input: "route to: default\ngateway: 192.0.2.1\n", want: ""},
		{name: "similarly named key does not match", input: "default_interface: en9\n", want: ""},
		{name: "empty value", input: "interface:   \n", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseRouteInterface(tt.input); got != tt.want {
				t.Fatalf("parseRouteInterface(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseRouteInterfaceRemovesTerminalControls(t *testing.T) {
	t.Parallel()

	got := parseRouteInterface("interface: en0\x1b[31m\r\n")
	if got == "" {
		t.Fatal("parseRouteInterface unexpectedly discarded the interface")
	}
	if strings.IndexFunc(got, unicode.IsControl) >= 0 {
		t.Fatalf("parseRouteInterface left control characters in %q", got)
	}
}
