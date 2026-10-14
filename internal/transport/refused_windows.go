package transport

import "syscall"

// errConnRefused is WSAECONNREFUSED. syscall.ECONNREFUSED on Windows is an
// invented value that a refused dial never returns.
const errConnRefused = syscall.Errno(10061)
