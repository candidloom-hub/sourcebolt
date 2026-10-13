// Package session owns everything about a signed-in NotebookLM session:
// which host the account uses, the page tokens, cookies and their storage.
package session

import "net/url"

// Host is a NotebookLM web host, optionally with a port (tests).
type Host string

// HostDefault is where rebranded accounts land since July 2026.
const HostDefault Host = "notebook.google.com"

var notebookHosts = map[string]bool{
	"notebook.google.com":         true,
	"notebooklm.google.com":       true,
	"notebook.cloud.google.com":   true,
	"notebooklm.cloud.google.com": true,
}

// IsNotebookHost is the only host check in the codebase. Upstreams that
// scattered host checks broke in five places on the rebrand.
func IsNotebookHost(u *url.URL) bool {
	return u != nil && notebookHosts[u.Hostname()]
}

// Origin returns "https://<host>".
func (h Host) Origin() string { return "https://" + string(h) }
