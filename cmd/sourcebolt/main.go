// Command sourcebolt is a fast MCP server for Google NotebookLM.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/candidloom-hub/sourcebolt/internal/version"
)

const usage = `sourcebolt - fast MCP server for Google NotebookLM

Usage:
  sourcebolt serve  [--profile p] [--tools default|minimal|all] [--http 127.0.0.1:PORT --token-file f]
  sourcebolt call   <tool> '<json args>'
  sourcebolt login  --import <file|-> [--format netscape|json|header] [--profile p] [--host h]
  sourcebolt logout [--profile p]
  sourcebolt status
  sourcebolt doctor
  sourcebolt --version

Environment: NOTEBOOKLM_PROFILE, NOTEBOOKLM_HOST, NOTEBOOKLM_CONFIG_DIR, NOTEBOOKLM_RPC_IDS
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is the testable entry point. Exit codes: 0 ok, 1 failure, 2 usage.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "--version", "version":
		fmt.Fprintln(stdout, version.String())
		return 0
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	case "serve":
		return cmdServe(ctx, rest, stdin, stdout, stderr)
	case "call":
		return cmdCall(ctx, rest, stdout, stderr)
	case "login":
		return cmdLogin(ctx, rest, stdin, stdout, stderr)
	case "logout":
		return cmdLogout(rest, stdout, stderr)
	case "status":
		return cmdStatus(stdout, stderr)
	case "doctor":
		return cmdDoctor(ctx, stdout, stderr)
	}
	fmt.Fprintf(stderr, "unknown command %q\n\n%s", cmd, usage)
	return 2
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, "error:", err)
	return 1
}
