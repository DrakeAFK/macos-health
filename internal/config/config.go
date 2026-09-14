package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Settings struct {
	SystemProcesses bool   `json:"system_processes"`
	Ports           bool   `json:"ports"`
	Interval        string `json:"interval"`
	Theme           string `json:"theme"`
	Page            int    `json:"page"`
	Redact          bool   `json:"redact"`
	ASCII           bool   `json:"ascii"`
	NoColor         bool   `json:"no_color"`
	Sensors         bool   `json:"sensors"`
	Group           bool   `json:"group_processes"`
}

func Defaults() Settings { return Settings{Interval: "1s", Theme: "ocean", Sensors: true} }
func Path() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "macos-health", "config.json")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "macos-health", "config.json")
}
func (s Settings) Validate() error {
	d, e := time.ParseDuration(s.Interval)
	if e != nil || d < 250*time.Millisecond || d > time.Minute {
		return fmt.Errorf("interval must be between 250ms and 1m")
	}
	if s.Theme != "ocean" && s.Theme != "amber" && s.Theme != "violet" {
		return fmt.Errorf("theme must be ocean, amber or violet")
	}
	if s.Page < 0 || s.Page > 7 {
		return fmt.Errorf("page must be 0 through 7")
	}
	return nil
}
func Load(path string) (Settings, error) {
	s := Defaults()
	f, e := os.Open(path)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return s, e
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 65537))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&s); e != nil {
		return s, e
	}
	var tail any
	if e = decoder.Decode(&tail); e != io.EOF {
		return s, fmt.Errorf("config must contain exactly one JSON object")
	}
	return s, s.Validate()
}
func Save(path string, s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".config-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(append(b, '\n')); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}
