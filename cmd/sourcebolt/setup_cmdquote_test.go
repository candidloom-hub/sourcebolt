package main

import (
	"testing"

	"github.com/candidloom-hub/sourcebolt/internal/setup"
)

// cmd.exe expands %NAME% in the arguments of a .cmd/.bat file and cannot be
// told not to, so an argument with '%' must never go through such a CLI
// (code.cmd, an npm-installed claude.cmd): setup edits the file instead.
func TestCLICommandsAvoidBatchFilesForPercent(t *testing.T) {
	plain := setup.Entry{Type: "stdio", Command: `C:\Users\Ann\AppData\Local\Programs\sourcebolt\sourcebolt.exe`, Args: []string{"serve"}}
	pct := setup.Entry{Type: "stdio", Command: `C:\Users\50%off%PATH%\AppData\Local\Programs\sourcebolt\sourcebolt.exe`, Args: []string{"serve"}}
	pctArg := setup.Entry{Type: "stdio", Command: `C:\sb\sourcebolt.exe`, Args: []string{"serve", "--profile", "a%b"}}
	cases := []struct {
		name   string
		id     string
		cli    string
		e      setup.Entry
		useCLI bool
	}{
		{"code.cmd, plain path", "vscode", `C:\VS Code\bin\code.cmd`, plain, true},
		{"code.cmd, percent in path", "vscode", `C:\VS Code\bin\code.cmd`, pct, false},
		{"CODE.CMD upper case", "vscode", `C:\VS Code\bin\CODE.CMD`, pct, false},
		{"code.bat, percent in path", "vscode", `C:\VS Code\bin\code.bat`, pct, false},
		{"code binary, percent in path", "vscode", "/usr/bin/code", pct, true},
		{"claude.cmd, plain path", "claude-code", `C:\npm\claude.cmd`, plain, true},
		{"claude.cmd, percent in path", "claude-code", `C:\npm\claude.cmd`, pct, false},
		{"claude.cmd, percent in an arg", "claude-code", `C:\npm\claude.cmd`, pctArg, false},
		{"claude.exe, percent in path", "claude-code", `C:\Users\x\.local\bin\claude.exe`, pct, true},
	}
	for _, tc := range cases {
		c := setup.Client{ID: tc.id, CLI: tc.cli}
		got := cliCommands(c, tc.e, false) != nil
		if got != tc.useCLI {
			t.Errorf("%s: uses CLI = %v, want %v", tc.name, got, tc.useCLI)
		}
	}
}
