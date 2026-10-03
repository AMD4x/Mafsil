package config

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestConservativeDefaults(t *testing.T) {
	c := Default(t.TempDir())
	if c.AllowExec || c.AllowWrite {
		t.Fatal("unsafe default")
	}
	b, _ := json.Marshal(c)
	if _, e := Decode(b); e != nil {
		t.Fatal(e)
	}
}
func TestRejectInvalidConfiguration(t *testing.T) {
	c := Default(t.TempDir())
	for _, mutate := range []func(*Config){func(c *Config) { c.Workspace = "." }, func(c *Config) { c.Workspace = filepath.VolumeName(c.Workspace) + string(filepath.Separator) }, func(c *Config) { c.AllowExec = true }, func(c *Config) { c.Limits.Sessions = 0 }, func(c *Config) { c.Limits.FileBytes = 1 << 30 }, func(c *Config) { c.Limits.OutputBytes = 0 }} {
		x := c
		mutate(&x)
		if x.Validate() == nil {
			t.Fatalf("accepted %+v", x)
		}
	}
	for _, data := range []string{`{"unknown":true}`, `{} {}`, `null`, string(make([]byte, MaxConfigBytes+1))} {
		if _, e := Decode([]byte(data)); e == nil {
			t.Fatal("invalid config accepted")
		}
	}
}
