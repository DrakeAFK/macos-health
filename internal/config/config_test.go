package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadAndRejectInvalid(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "prefs.json")
	s := Defaults()
	s.Redact = true
	s.Theme = "amber"
	if e := Save(p, s); e != nil {
		t.Fatal(e)
	}
	got, e := Load(p)
	if e != nil || got != s {
		t.Fatalf("%+v %v", got, e)
	}
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0600 {
		t.Fatal("preferences are not private")
	}
	for _, text := range []string{`{"theme":"invalid"}`, `{"surprise":true}`, `{"interval":"1ms"}`, `{} {}`} {
		if e := os.WriteFile(p, []byte(text), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := Load(p); e == nil {
			t.Fatalf("accepted %s", text)
		}
	}
}
