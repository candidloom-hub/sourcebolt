//go:build !windows && !linux

package main

import "errors"

// peakRSS is not implemented here (macOS is skipped by design).
func peakRSS(int) (int64, error) { return 0, errors.New("unsupported platform") }

const supported = false

// rssDetail is only implemented on Linux.
func rssDetail(int) string { return "" }
