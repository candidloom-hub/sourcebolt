//go:build !linux

package thp

func disabled() bool { return false }
