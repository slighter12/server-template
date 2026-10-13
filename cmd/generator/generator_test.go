package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite expected_proxy.go files")

func TestGenerateSnapshots(t *testing.T) {
	cases := []struct{ name, iface, pkg string }{
		{"basic", "Store", "basicproxy"},
		{"variadic_any", "Logger", "variadicproxy"},
		{"noctx", "Helper", "noctxproxy"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join("testdata", tc.name)
			got, err := generate(Config{
				Source:     filepath.Join(dir, "interface.go"),
				Interface:  tc.iface,
				Package:    tc.pkg,
				Tracer:     "test-tracer",
				Template:   "otel",
				ModuleName: "server-template",
			})
			if err != nil {
				t.Fatal(err)
			}

			if bytes.Contains(got, []byte("interface{}")) {
				t.Error("output contains interface{}; want any")
			}
			for i, line := range strings.Split(string(got), "\n") {
				if line != strings.TrimRight(line, " \t") {
					t.Errorf("trailing whitespace on line %d", i+1)
				}
			}

			// The expected file lives in testdata (ignored by the go tool) with a .go name by design.
			want := filepath.Join(dir, "expected_proxy.go")
			if *update {
				if err := os.WriteFile(want, got, 0o644); err != nil {
					t.Fatal(err)
				}

				return
			}
			exp, err := os.ReadFile(want)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(exp, got) {
				t.Errorf("output differs from %s; run go test ./cmd/generator -update\n%s", want, got)
			}
		})
	}
}

func TestGenerateErrors(t *testing.T) {
	base := Config{Source: "testdata/basic/interface.go", Interface: "Store", Template: "otel", ModuleName: "server-template"}

	bad := base
	bad.Template = "nope"
	if _, err := generate(bad); err == nil {
		t.Error("expected unsupported template error")
	}

	bad = base
	bad.Interface = "Missing"
	if _, err := generate(bad); err == nil {
		t.Error("expected missing interface error")
	}
}
