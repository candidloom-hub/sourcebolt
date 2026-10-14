//go:build !windows

package transport

import "syscall"

const errConnRefused = syscall.ECONNREFUSED
