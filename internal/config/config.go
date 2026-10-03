// Package config defines the operator-controlled capability boundary.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const MaxConfigBytes = 64 << 10

type Config struct {
	Workspace        string `json:"workspace"`
	AllowWrite       bool   `json:"allowWrite"`
	AllowExec        bool   `json:"allowExec"`
	Shell            string `json:"shell,omitempty"`
	ScratchDirectory string `json:"scratchDirectory,omitempty"`
	Limits           Limits `json:"limits"`
}

type Limits struct {
	FileBytes        int `json:"fileBytes"`
	ReadBytes        int `json:"readBytes"`
	OutputBytes      int `json:"outputBytes"`
	Sessions         int `json:"sessions"`
	RetainedSessions int `json:"retainedSessions"`
	TimeoutSeconds   int `json:"timeoutSeconds"`
	IdleSeconds      int `json:"idleSeconds"`
}

func Default(workspace string) Config {
	return Config{Workspace: workspace, Limits: Limits{FileBytes: 4 << 20, ReadBytes: 256 << 10, OutputBytes: 256 << 10, Sessions: 8, RetainedSessions: 16, TimeoutSeconds: 900, IdleSeconds: 300}}
}

func Decode(data []byte) (Config, error) {
	c := Default("")
	if len(data) > MaxConfigBytes {
		return c, errors.New("configuration exceeds 64 KiB")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, fmt.Errorf("invalid configuration: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return c, errors.New("configuration must contain one JSON object")
	}
	return c, c.Validate()
}

func Load(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Config{}, err
	}
	if !info.Mode().IsRegular() {
		return Config{}, errors.New("configuration must be a regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxConfigBytes+1))
	if err != nil {
		return Config{}, err
	}
	return Decode(b)
}

func (c Config) Validate() error {
	if !filepath.IsAbs(c.Workspace) || filepath.Clean(c.Workspace) == filepath.VolumeName(c.Workspace)+string(filepath.Separator) {
		return errors.New("workspace must be an absolute directory below a filesystem root")
	}
	if c.Shell != "" && !filepath.IsAbs(c.Shell) {
		return errors.New("shell must be an absolute executable path")
	}
	if c.AllowExec && (!filepath.IsAbs(c.ScratchDirectory) || filepath.Clean(c.ScratchDirectory) == filepath.Clean(c.Workspace)) {
		return errors.New("execution requires a separate absolute scratchDirectory")
	}
	l := c.Limits
	if l.FileBytes < 1024 || l.FileBytes > 16<<20 || l.ReadBytes < 256 || l.ReadBytes > 1<<20 || l.ReadBytes > l.FileBytes {
		return errors.New("fileBytes must be 1 KiB–16 MiB; readBytes 256 B–1 MiB and no larger than fileBytes")
	}
	if l.OutputBytes < 1024 || l.OutputBytes > 1<<20 || l.Sessions < 1 || l.Sessions > 16 || l.RetainedSessions < 0 || l.RetainedSessions > 64 {
		return errors.New("invalid output or session limits")
	}
	if l.TimeoutSeconds < 1 || l.TimeoutSeconds > 3600 || l.IdleSeconds < 1 || l.IdleSeconds > 3600 {
		return errors.New("timeoutSeconds and idleSeconds must be 1–3600")
	}
	return nil
}
