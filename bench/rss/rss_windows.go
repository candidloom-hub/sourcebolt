//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var getProcessMemoryInfo = syscall.NewLazyDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

// processMemoryCounters mirrors PROCESS_MEMORY_COUNTERS.
type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

// peakRSS returns the peak working set of pid in bytes (pid 0 = this process).
func peakRSS(pid int) (int64, error) {
	var h syscall.Handle
	if pid == 0 {
		var err error
		if h, err = syscall.GetCurrentProcess(); err != nil {
			return 0, err
		}
	} else {
		const queryInformation, vmRead = 0x0400, 0x0010
		var err error
		if h, err = syscall.OpenProcess(queryInformation|vmRead, false, uint32(pid)); err != nil {
			return 0, err
		}
		defer syscall.CloseHandle(h)
	}
	var c processMemoryCounters
	c.cb = uint32(unsafe.Sizeof(c))
	r, _, err := getProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&c)), uintptr(c.cb))
	if r == 0 {
		return 0, fmt.Errorf("GetProcessMemoryInfo: %w", err)
	}
	return int64(c.peakWorkingSetSize), nil
}

// supported reports whether this platform can measure RSS.
const supported = true

// rssDetail is only implemented on Linux.
func rssDetail(int) string { return "" }
