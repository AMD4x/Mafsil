package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	var b bytes.Buffer
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "config.json")
	for _, args := range [][]string{{"--version"}, {"help"}, {"init", "--help"}, {"serve", "--help"}, {"init", "--workspace", root, "--output", path}, {"doctor", "--config", path}} {
		if e := run(args, &b, &b); e != nil {
			t.Fatalf("%v: %v", args, e)
		}
	}
	if !strings.Contains(b.String(), `"ok":true`) {
		t.Fatal(b.String())
	}
	before, _ := os.ReadFile(path)
	if e := run([]string{"init", "--workspace", root, "--output", path}, &b, &b); e == nil {
		t.Fatal("init clobbered")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("config changed")
	}
	for _, args := range [][]string{{"serve"}, {"invalid"}, {"doctor", "--config", path, "extra"}} {
		if e := run(args, &b, &b); e == nil {
			t.Fatal("bad CLI accepted", args)
		}
	}
}
