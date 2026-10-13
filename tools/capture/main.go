// Command capture converts a HAR recorded from the NotebookLM web app into
// sanitized golden fixtures under testdata/fixtures/<rpc>/.
//
//	go run ./tools/capture --har capture.har --out testdata/fixtures [--keep-titles]
//
// Never commit the HAR itself: it holds session cookies.
package main

import (
	"encoding/json/v2"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	harPath := flag.String("har", "", "HAR file exported from the browser")
	outDir := flag.String("out", "testdata/fixtures", "fixture root")
	keep := flag.Bool("keep-titles", false, "keep notebook titles and other free text")
	flag.Parse()
	if *harPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	raw, err := os.ReadFile(*harPath)
	if err != nil {
		fatal(err)
	}
	fixtures, err := Convert(raw, Options{KeepTitles: *keep})
	if err != nil {
		fatal(err)
	}
	seq := map[string]int{}
	for _, f := range fixtures {
		seq[f.RPC]++
		dir := filepath.Join(*outDir, f.RPC)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fatal(err)
		}
		name := fmt.Sprintf("live_%s_%02d.json", f.Captured, seq[f.RPC])
		b, err := json.Marshal(f, json.Deterministic(true))
		if err != nil {
			fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0o644); err != nil {
			fatal(err)
		}
		fmt.Println(filepath.Join(dir, name))
	}
	fmt.Fprintf(os.Stderr, "%d fixtures written\n", len(fixtures))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "capture:", err)
	os.Exit(1)
}
