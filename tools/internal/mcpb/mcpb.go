// Package mcpb builds MCP Bundles (.mcpb, manifest_version 0.3) for
// sourcebolt: one bundle per OS/arch, each a zip holding manifest.json, the
// native binary under server/, icon.png, LICENSE and NOTICE.
//
// Spec: https://github.com/modelcontextprotocol/mcpb/blob/main/MANIFEST.md
// The manifest is rendered from packaging/mcpb/manifest.json; the tool list
// comes from the real server so it cannot drift from the code.
package mcpb

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"
	"time"

	"github.com/candidloom-hub/sourcebolt/internal/cache"
	"github.com/candidloom-hub/sourcebolt/internal/mcpserver"
	"github.com/candidloom-hub/sourcebolt/internal/nlm"
	"github.com/candidloom-hub/sourcebolt/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Platform is a Go OS/arch pair that has a release binary.
type Platform struct{ GOOS, GOARCH string }

// Platforms are the six GoReleaser targets (see .goreleaser.yaml).
var Platforms = []Platform{
	{"linux", "amd64"}, {"linux", "arm64"},
	{"darwin", "amd64"}, {"darwin", "arm64"},
	{"windows", "amd64"}, {"windows", "arm64"},
}

// MCPBOS is the platform name the MCPB spec uses (Node's process.platform).
func (p Platform) MCPBOS() string {
	if p.GOOS == "windows" {
		return "win32"
	}
	return p.GOOS
}

// Binary is the executable's file name on this platform.
func (p Platform) Binary() string {
	if p.GOOS == "windows" {
		return "sourcebolt.exe"
	}
	return "sourcebolt"
}

// FileName is the release asset name. The MCP registry requires "mcp" in an
// MCPB download URL; the repository name lacks it, so the file carries it.
func (p Platform) FileName() string {
	return fmt.Sprintf("sourcebolt-mcp-%s-%s.mcpb", p.GOOS, p.GOARCH)
}

// Tool is a manifest tools[] entry.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

var semver = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

// Manifest renders the template for one platform.
func Manifest(template []byte, version string, p Platform, tools []Tool) ([]byte, error) {
	if !semver.MatchString(version) {
		return nil, fmt.Errorf("version %q is not semver (no leading v)", version)
	}
	var m map[string]any
	if err := json.Unmarshal(template, &m); err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}
	server, ok := m["server"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("template: no server object")
	}
	cfg, ok := server["mcp_config"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("template: no server.mcp_config object")
	}
	m["version"] = version
	server["entry_point"] = "server/" + p.Binary()
	// Hosts append .exe on Windows for binary servers, but naming the file
	// exactly does not depend on that behavior.
	cfg["command"] = "${__dirname}/server/" + p.Binary()
	m["compatibility"] = map[string]any{"platforms": []string{p.MCPBOS()}}
	m["tools"] = tools
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Files are the bundle contents besides manifest.json.
type Files struct {
	Binary  []byte
	Icon    []byte // PNG
	License []byte
	Notice  []byte
}

// epoch is the fixed timestamp of every zip entry, so a bundle's bytes depend
// only on its inputs.
var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// Pack writes the bundle zip to w.
func Pack(w io.Writer, manifest []byte, p Platform, f Files) error {
	if len(f.Binary) == 0 {
		return fmt.Errorf("empty binary")
	}
	zw := zip.NewWriter(w)
	add := func(name string, data []byte, mode uint32) error {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: epoch}
		h.SetMode(fs.FileMode(mode))
		fw, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = fw.Write(data)
		return err
	}
	entries := []struct {
		name string
		data []byte
		mode uint32
	}{
		{"manifest.json", manifest, 0o644},
		{"server/" + p.Binary(), f.Binary, 0o755},
		{"icon.png", f.Icon, 0o644},
		{"LICENSE", f.License, 0o644},
		{"NOTICE", f.Notice, 0o644},
	}
	for _, e := range entries {
		if e.data == nil {
			continue
		}
		if err := add(e.name, e.data, e.mode); err != nil {
			return err
		}
	}
	return zw.Close()
}

// Tools lists the default-profile tools of the real server, in-process, with
// a transport that refuses every call (no network, no credentials).
func Tools(ctx context.Context) ([]Tool, error) {
	srv := mcpserver.New(mcpserver.Deps{
		NLM:     nlm.New(noNet{}),
		Cache:   cache.New(nil),
		Profile: "default",
		Host:    "notebook.google.com",
	}, "default")
	ct, st := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		return nil, err
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "mcpb", Version: "v0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		return nil, err
	}
	defer cs.Close()
	var out []Tool
	for t, err := range cs.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		out = append(out, Tool{Name: t.Name, Description: firstSentence(t.Description)})
	}
	return out, nil
}

// firstSentence keeps manifests short: the first sentence, one line.
func firstSentence(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if i := strings.Index(s, ". "); i > 0 {
		s = s[:i+1]
	}
	return s
}

type noNet struct{}

func (noNet) Name() string         { return "none" }
func (noNet) Supports(string) bool { return false }
func (noNet) Do(context.Context, transport.Call) (transport.Result, error) {
	return transport.Result{}, fmt.Errorf("mcpb: network disabled")
}
