// Command sourcebolt is a fast MCP server for Google NotebookLM.
package main

import (
	"fmt"
	"os"

	"github.com/candidloom-hub/sourcebolt/internal/version"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println(version.String())
		return
	}
	fmt.Fprintln(os.Stderr, "usage: sourcebolt --version")
	os.Exit(2)
}
