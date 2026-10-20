//go:build linux

package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// peakRSS returns the peak resident set (VmHWM) of pid in bytes (pid 0 =
// this process).
func peakRSS(pid int) (int64, error) {
	path := "/proc/self/status"
	if pid != 0 {
		path = fmt.Sprintf("/proc/%d/status", pid)
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), "VmHWM:"); ok {
			fields := strings.Fields(rest) // "<n> kB"
			if len(fields) == 0 {
				break
			}
			kb, err := strconv.ParseInt(fields[0], 10, 64)
			return kb * 1024, err
		}
	}
	return 0, fmt.Errorf("VmHWM not found in %s", path)
}

const supported = true

// rssDetail returns the resident-memory breakdown of pid (anonymous pages,
// that is heap and stacks, versus file pages, mostly the binary's own code).
func rssDetail(pid int) string {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return ""
	}
	var parts []string
	for _, l := range strings.Split(string(b), "\n") {
		for _, k := range []string{"VmHWM:", "VmRSS:", "RssAnon:", "RssFile:"} {
			if rest, ok := strings.CutPrefix(l, k); ok {
				parts = append(parts, k+" "+strings.TrimSpace(rest))
			}
		}
	}
	return strings.Join(parts, ", ")
}
