// Command icon writes the sourcebolt mark as SVG or PNG.
//
//	go run ./tools/icon -svg > docs/assets/icon.svg
//	go run ./tools/icon -png 512 -o dist/icon-512.png   # MCPB, directory listings
//	go run ./tools/icon -png 400 -o dist/icon-400.png   # Cline marketplace
//
// PNGs are generated, not committed: the output is deterministic, and the
// release workflow builds the 512 px icon into each .mcpb bundle.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/candidloom-hub/sourcebolt/tools/internal/icon"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "icon:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("icon", flag.ContinueOnError)
	svg := fs.Bool("svg", false, "write SVG")
	size := fs.Int("png", 0, "write a PNG of this many pixels square (16..2048)")
	out := fs.String("o", "", "output file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var data []byte
	switch {
	case *svg && *size == 0:
		data = icon.SVG()
	case !*svg && *size >= 16 && *size <= 2048:
		var err error
		if data, err = icon.PNG(*size); err != nil {
			return err
		}
	default:
		return fmt.Errorf("use exactly one of -svg or -png <16..2048>")
	}
	if *out == "" {
		_, err := stdout.Write(data)
		return err
	}
	return os.WriteFile(*out, data, 0o644)
}
