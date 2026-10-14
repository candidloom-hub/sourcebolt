// Command coldstart measures B1: time from process spawn to the first
// `initialize` response line of `<bin> serve`.
//
//	go run ./bench/coldstart -bin bin/sourcebolt.exe [-runs 30] [-check]
//
// It builds nothing; pass an already built binary. Each run uses an empty
// temporary config dir and an unreachable host (127.0.0.1:1), so the server
// can never touch a real profile or the network. The first run is dropped
// (cold file cache, antivirus scan).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

const initRequest = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"coldstart","version":"0"}}}` + "\n"

func main() {
	bin := flag.String("bin", "", "path to the sourcebolt binary (required)")
	runs := flag.Int("runs", 30, "number of runs; the first is dropped")
	check := flag.Bool("check", false, "exit 1 unless p50 < 30 ms and p95 < 60 ms")
	flag.Parse()
	if *bin == "" || *runs < 2 {
		flag.Usage()
		os.Exit(2)
	}
	var times []time.Duration
	for i := range *runs {
		d, err := once(*bin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "run %d: %v\n", i, err)
			os.Exit(1)
		}
		if i > 0 {
			times = append(times, d)
		}
	}
	slices.Sort(times)
	p50, p95 := pct(times, 50), pct(times, 95)
	fmt.Printf("B1 cold start to initialize (%d runs, first dropped)\n", len(times))
	fmt.Printf("min=%s p50=%s p95=%s max=%s\n", ms(times[0]), ms(p50), ms(p95), ms(times[len(times)-1]))
	fmt.Println("target: p50 < 30 ms, p95 < 60 ms")
	if *check && (p50 >= 30*time.Millisecond || p95 >= 60*time.Millisecond) {
		fmt.Println("FAIL")
		os.Exit(1)
	}
}

func once(bin string) (time.Duration, error) {
	dir, err := os.MkdirTemp("", "sb-coldstart-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	cmd := exec.Command(bin, "serve")
	cmd.Env = append(os.Environ(), "NOTEBOOKLM_CONFIG_DIR="+dir, "NOTEBOOKLM_HOST=127.0.0.1:1")
	in, err := cmd.StdinPipe()
	if err != nil {
		return 0, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	defer func() {
		in.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()
	if _, err := io.WriteString(in, initRequest); err != nil {
		return 0, err
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	d := time.Since(start)
	if err != nil {
		return 0, fmt.Errorf("reading initialize response: %w", err)
	}
	if !strings.Contains(line, `"result"`) || !strings.Contains(line, "serverInfo") {
		return 0, fmt.Errorf("unexpected initialize response: %.200s", line)
	}
	return d, nil
}

func pct(sorted []time.Duration, p int) time.Duration {
	return sorted[min(len(sorted)*p/100, len(sorted)-1)]
}

func ms(d time.Duration) string { return fmt.Sprintf("%.1f ms", float64(d)/float64(time.Millisecond)) }
