package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/candidloom-hub/sourcebolt/tools/internal/mcpb"
	"github.com/candidloom-hub/sourcebolt/tools/internal/schemas"
)

const root = "../.."

func fakeDist(t *testing.T) string {
	t.Helper()
	dist := t.TempDir()
	dirs := map[string]string{
		"linux-amd64": "sourcebolt_linux_amd64_v1", "linux-arm64": "sourcebolt_linux_arm64_v8.0",
		"darwin-amd64": "sourcebolt_darwin_amd64_v1", "darwin-arm64": "sourcebolt_darwin_arm64_v8.0",
		"windows-amd64": "sourcebolt_windows_amd64_v1", "windows-arm64": "sourcebolt_windows_arm64_v8.0",
	}
	for _, p := range mcpb.Platforms {
		d := filepath.Join(dist, dirs[p.GOOS+"-"+p.GOARCH])
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, p.Binary()), []byte("bin-"+p.GOOS+"-"+p.GOARCH), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A browser build must never be picked.
	b := filepath.Join(dist, "sourcebolt-browser_linux_amd64_v1")
	_ = os.MkdirAll(b, 0o755)
	_ = os.WriteFile(filepath.Join(b, "sourcebolt"), []byte("WRONG"), 0o755)
	return dist
}

func TestBuildsValidBundlesForEveryPlatform(t *testing.T) {
	dist := fakeDist(t)
	out := filepath.Join(dist, "mcpb")
	var log bytes.Buffer
	if err := run(context.Background(), []string{"--version", "v1.2.3", "--dist", dist, "--root", root}, &log); err != nil {
		t.Fatal(err)
	}
	sums, err := os.ReadFile(filepath.Join(out, "sourcebolt-mcp-checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range mcpb.Platforms {
		name := p.FileName()
		raw, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(raw)
		if !strings.Contains(string(sums), hex.EncodeToString(h[:])+"  "+name+"\n") {
			t.Errorf("%s: checksum line missing", name)
		}
		zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			t.Fatal(err)
		}
		files := map[string][]byte{}
		for _, f := range zr.File {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			files[f.Name] = b
			if f.Name == "server/"+p.Binary() && f.Mode().Perm() != 0o755 {
				t.Errorf("%s: binary mode %v", name, f.Mode())
			}
		}
		for _, want := range []string{"manifest.json", "server/" + p.Binary(), "icon.png", "LICENSE", "NOTICE"} {
			if _, ok := files[want]; !ok {
				t.Errorf("%s: missing %s", name, want)
			}
		}
		if got := string(files["server/"+p.Binary()]); got != "bin-"+p.GOOS+"-"+p.GOARCH {
			t.Errorf("%s: wrong binary %q", name, got)
		}
		man := files["manifest.json"]
		if err := schemas.Validate(schemas.MCPBManifest, man); err != nil {
			t.Fatalf("%s: manifest fails the MCPB v0.3 schema: %v", name, err)
		}
		var m struct {
			Version string `json:"version"`
			Server  struct {
				EntryPoint string `json:"entry_point"`
				MCPConfig  struct {
					Command string   `json:"command"`
					Args    []string `json:"args"`
				} `json:"mcp_config"`
			} `json:"server"`
			Compat struct {
				Platforms []string `json:"platforms"`
			} `json:"compatibility"`
			Tools []mcpb.Tool `json:"tools"`
		}
		if err := json.Unmarshal(man, &m); err != nil {
			t.Fatal(err)
		}
		if m.Version != "1.2.3" || m.Server.EntryPoint != "server/"+p.Binary() ||
			m.Server.MCPConfig.Command != "${__dirname}/server/"+p.Binary() ||
			strings.Join(m.Server.MCPConfig.Args, " ") != "serve" ||
			len(m.Compat.Platforms) != 1 || m.Compat.Platforms[0] != p.MCPBOS() {
			t.Errorf("%s: manifest fields wrong: %+v", name, m)
		}
		if _, ok := files[m.Server.EntryPoint]; !ok {
			t.Errorf("%s: entry_point %s not in bundle", name, m.Server.EntryPoint)
		}
		if len(m.Tools) < 20 {
			t.Errorf("%s: only %d tools listed", name, len(m.Tools))
		}
	}
}

func TestBundlesAreDeterministic(t *testing.T) {
	dist := fakeDist(t)
	build := func(out string) []byte {
		if err := run(context.Background(), []string{"--version", "0.1.0", "--dist", dist, "--root", root, "--out", out, "--platforms", "linux-amd64"}, io.Discard); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(out, "sourcebolt-mcp-linux-amd64.mcpb"))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	a, b := build(filepath.Join(dist, "a")), build(filepath.Join(dist, "b"))
	if !bytes.Equal(a, b) {
		t.Fatal("two builds from the same inputs differ")
	}
}

func TestMissingBinaryFailsUnlessAllowed(t *testing.T) {
	dist := t.TempDir()
	if err := run(context.Background(), []string{"--version", "0.1.0", "--dist", dist, "--root", root}, io.Discard); err == nil {
		t.Fatal("expected an error for a missing binary")
	}
	if err := run(context.Background(), []string{"--version", "0.1.0", "--dist", dist, "--root", root, "--allow-missing"}, io.Discard); err == nil {
		t.Fatal("expected an error when nothing was built")
	}
	if err := run(context.Background(), []string{"--version", "latest", "--dist", fakeDist(t), "--root", root}, io.Discard); err == nil {
		t.Fatal("expected an error for a non-semver version")
	}
}

// The committed template must itself satisfy the schema.
func TestTemplateIsValid(t *testing.T) {
	tpl, err := os.ReadFile(filepath.Join(root, "packaging", "mcpb", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := schemas.Validate(schemas.MCPBManifest, tpl); err != nil {
		t.Fatal(err)
	}
}
