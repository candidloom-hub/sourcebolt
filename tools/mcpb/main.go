// Command mcpb builds one MCP Bundle per platform from GoReleaser's dist/.
// It never publishes; the release workflow uploads the files to the draft
// GitHub release on a tag push.
//
//	go run ./tools/mcpb --version 0.1.0 [--dist dist] [--out dist/mcpb] [--root .]
//	                    [--platforms linux-amd64,windows-amd64] [--allow-missing]
//
// Output: <out>/sourcebolt-mcp-<goos>-<goarch>.mcpb and
// <out>/sourcebolt-mcp-checksums.txt (sha256, the format of checksums.txt;
// these hashes go into server.json's fileSha256 for registryType "mcpb").
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/candidloom-hub/sourcebolt/tools/internal/icon"
	"github.com/candidloom-hub/sourcebolt/tools/internal/mcpb"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "mcpb:", err)
		os.Exit(1)
	}
}

type options struct {
	version, dist, out, root string
	platforms                []mcpb.Platform
	allowMissing             bool
}

func parse(args []string) (options, error) {
	fs := flag.NewFlagSet("mcpb", flag.ContinueOnError)
	o := options{}
	fs.StringVar(&o.version, "version", "", "bundle version (semver, a leading v is stripped)")
	fs.StringVar(&o.dist, "dist", "dist", "GoReleaser dist directory")
	fs.StringVar(&o.out, "out", "", "output directory (default <dist>/mcpb)")
	fs.StringVar(&o.root, "root", ".", "repository root (manifest template, LICENSE, NOTICE)")
	plats := fs.String("platforms", "", "comma-separated goos-goarch list (default: all six)")
	fs.BoolVar(&o.allowMissing, "allow-missing", false, "skip platforms whose binary is not in dist")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	o.version = strings.TrimPrefix(o.version, "v")
	if o.version == "" {
		return o, fmt.Errorf("--version is required")
	}
	if o.out == "" {
		o.out = filepath.Join(o.dist, "mcpb")
	}
	if *plats == "" {
		o.platforms = mcpb.Platforms
		return o, nil
	}
	for _, s := range strings.Split(*plats, ",") {
		var found bool
		for _, p := range mcpb.Platforms {
			if p.GOOS+"-"+p.GOARCH == s {
				o.platforms, found = append(o.platforms, p), true
			}
		}
		if !found {
			return o, fmt.Errorf("unknown platform %q", s)
		}
	}
	return o, nil
}

// findBinary locates dist/sourcebolt_<goos>_<goarch>[_v1|_v8.0]/<binary>.
// The browser build (sourcebolt-browser_*) never matches.
func findBinary(dist string, p mcpb.Platform) (string, error) {
	prefix := "sourcebolt_" + p.GOOS + "_" + p.GOARCH
	ents, err := os.ReadDir(dist)
	if err != nil {
		return "", err
	}
	for _, e := range ents {
		if e.IsDir() && (e.Name() == prefix || strings.HasPrefix(e.Name(), prefix+"_")) {
			f := filepath.Join(dist, e.Name(), p.Binary())
			if _, err := os.Stat(f); err == nil {
				return f, nil
			}
		}
	}
	return "", os.ErrNotExist
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	o, err := parse(args)
	if err != nil {
		return err
	}
	tpl, err := os.ReadFile(filepath.Join(o.root, "packaging", "mcpb", "manifest.json"))
	if err != nil {
		return err
	}
	license, err := os.ReadFile(filepath.Join(o.root, "LICENSE"))
	if err != nil {
		return err
	}
	notice, err := os.ReadFile(filepath.Join(o.root, "NOTICE"))
	if err != nil {
		return err
	}
	png, err := icon.PNG(512)
	if err != nil {
		return err
	}
	tools, err := mcpb.Tools(ctx)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(o.out, 0o755); err != nil {
		return err
	}
	var sums strings.Builder
	built := 0
	for _, p := range o.platforms {
		bin, err := findBinary(o.dist, p)
		if err != nil {
			if o.allowMissing {
				fmt.Fprintf(stdout, "skip %s-%s: no binary in %s\n", p.GOOS, p.GOARCH, o.dist)
				continue
			}
			return fmt.Errorf("%s-%s: binary not found in %s", p.GOOS, p.GOARCH, o.dist)
		}
		data, err := os.ReadFile(bin)
		if err != nil {
			return err
		}
		man, err := mcpb.Manifest(tpl, o.version, p, tools)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := mcpb.Pack(&buf, man, p, mcpb.Files{Binary: data, Icon: png, License: license, Notice: notice}); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(o.out, p.FileName()), buf.Bytes(), 0o644); err != nil {
			return err
		}
		h := sha256.Sum256(buf.Bytes())
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(h[:]), p.FileName())
		fmt.Fprintf(stdout, "%s: %d bytes\n", p.FileName(), buf.Len())
		built++
	}
	if built == 0 {
		return fmt.Errorf("no bundles built")
	}
	return os.WriteFile(filepath.Join(o.out, "sourcebolt-mcp-checksums.txt"), []byte(sums.String()), 0o644)
}
