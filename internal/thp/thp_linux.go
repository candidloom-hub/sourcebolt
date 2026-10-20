//go:build linux

package thp

import "syscall"

const (
	prSetTHPDisable = 41 // PR_SET_THP_DISABLE
	prGetTHPDisable = 42 // PR_GET_THP_DISABLE
)

func init() {
	// Best effort: a kernel without the option leaves THP as configured.
	_, _, _ = syscall.RawSyscall(syscall.SYS_PRCTL, prSetTHPDisable, 1, 0)
}

func disabled() bool {
	r, _, e := syscall.RawSyscall(syscall.SYS_PRCTL, prGetTHPDisable, 0, 0)
	return e == 0 && r == 1
}
