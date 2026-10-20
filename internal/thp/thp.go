// Package thp turns off transparent huge pages for this process on Linux,
// when it is imported (cmd/sourcebolt imports it for its side effect).
//
// With the kernel's THP mode "always" (common on cloud VMs, including
// GitHub's ubuntu-latest), the Go heap's first pages are backed by 2 MB
// huge pages: the idle server measured 9.2 to 9.8 MB anonymous RSS instead
// of 5.8 MB (see go.dev/issue/64332). The server's heap is a few MB, so huge
// pages buy nothing. The runtime's own switch (GODEBUG=disablethp=1) cannot
// be set from go.mod or //go:debug, so this uses prctl(PR_SET_THP_DISABLE),
// which the kernel applies to later page faults. Child processes (the
// fallback browser) inherit it.
package thp

// Disabled reports whether huge pages are off for this process. It is
// always false where the switch does not exist (non-Linux).
func Disabled() bool { return disabled() }
