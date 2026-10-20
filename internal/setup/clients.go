package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Env is what client discovery reads from the machine. Tests build one
// without touching the real home directory.
type Env struct {
	GOOS      string
	Home      string                       // user home directory
	AppData   string                       // %APPDATA% (Windows)
	XDGConfig string                       // $XDG_CONFIG_HOME; empty = <home>/.config
	Exists    func(path string) bool       // file or directory exists
	LookPath  func(string) (string, error) // finds a program on PATH

	LocalAppData   string // %LOCALAPPDATA% (Windows installer target)
	TempDir        string // the OS temp dir (go run, unpacked downloads)
	HomebrewPrefix string // $HOMEBREW_PREFIX
}

// OSEnv reads the real machine.
func OSEnv() Env {
	home, _ := os.UserHomeDir()
	return Env{
		GOOS:      runtime.GOOS,
		Home:      home,
		AppData:   os.Getenv("APPDATA"),
		XDGConfig: os.Getenv("XDG_CONFIG_HOME"),
		Exists: func(p string) bool {
			_, err := os.Stat(p)
			return err == nil
		},
		LookPath: exec.LookPath,

		LocalAppData:   os.Getenv("LOCALAPPDATA"),
		TempDir:        os.TempDir(),
		HomebrewPrefix: os.Getenv("HOMEBREW_PREFIX"),
	}
}

func (e Env) xdg() string {
	if e.XDGConfig != "" {
		return e.XDGConfig
	}
	return filepath.Join(e.Home, ".config")
}

// appSupport is the per-user application data dir of desktop apps:
// %APPDATA% on Windows, ~/Library/Application Support on macOS and the XDG
// config dir on Linux.
func (e Env) appSupport() string {
	switch e.GOOS {
	case "windows":
		return e.AppData
	case "darwin":
		return filepath.Join(e.Home, "Library", "Application Support")
	}
	return e.xdg()
}

func (e Env) look(name string) string {
	if e.LookPath == nil {
		return ""
	}
	p, err := e.LookPath(name)
	if err != nil {
		return ""
	}
	return p
}

// Client is one MCP app and where its user-level config lives.
type Client struct {
	ID        string // --client name
	Name      string // shown to the user
	Path      string // user-level config file
	Key       string // top-level key that holds the servers
	Typed     bool   // entries carry "type": "stdio"
	CLI       string // the app's own CLI, when on PATH and preferred over editing the file
	Installed bool
	Docs      string // where the path and shape come from
}

// Entry is the sourcebolt server entry for this client, running command
// with args (see ServerCommand).
func (c Client) Entry(command string, args []string) Entry {
	e := Entry{Command: command, Args: append([]string(nil), args...)}
	if c.Typed {
		e.Type = "stdio"
	}
	return e
}

// IDs lists the client ids in the order setup asks about them.
var IDs = []string{"claude-desktop", "claude-code", "cursor", "vscode", "windsurf", "gemini"}

// Clients lists every supported client with its config path for env.
func Clients(env Env) []Client {
	app := env.appSupport()

	// Claude Desktop: Settings > Developer > Edit Config opens
	// %APPDATA%\Claude\claude_desktop_config.json (Windows) or
	// ~/Library/Application Support/Claude/claude_desktop_config.json (macOS),
	// top-level "mcpServers". Source:
	// https://modelcontextprotocol.io/docs/develop/connect-local-servers
	// The Linux path (~/.config/Claude) is not on that page: unverified.
	desktopDir := filepath.Join(app, "Claude")
	desktop := Client{
		ID: "claude-desktop", Name: "Claude Desktop",
		Path: filepath.Join(desktopDir, "claude_desktop_config.json"), Key: "mcpServers",
		Installed: env.Exists(desktopDir),
		Docs:      "https://modelcontextprotocol.io/docs/develop/connect-local-servers",
	}

	// Claude Code: `claude mcp add --scope user --transport stdio <name> --
	// <command> [args...]`; user scope lives in ~/.claude.json. Source:
	// https://code.claude.com/docs/en/mcp
	// The top-level "mcpServers" key in ~/.claude.json is inferred from that
	// page ("the top level of ~/.claude.json"), so the CLI is preferred.
	codeJSON := filepath.Join(env.Home, ".claude.json")
	claudeCLI := env.look("claude")
	claudeCode := Client{
		ID: "claude-code", Name: "Claude Code",
		Path: codeJSON, Key: "mcpServers", Typed: true, CLI: claudeCLI,
		Installed: claudeCLI != "" || env.Exists(codeJSON),
		Docs:      "https://code.claude.com/docs/en/mcp",
	}

	// Cursor: ~/.cursor/mcp.json for all projects, top-level "mcpServers";
	// the field table lists "type": "stdio" for local servers. Source:
	// https://cursor.com/docs/context/mcp
	cursorDir := filepath.Join(env.Home, ".cursor")
	cursor := Client{
		ID: "cursor", Name: "Cursor",
		Path: filepath.Join(cursorDir, "mcp.json"), Key: "mcpServers", Typed: true,
		Installed: env.Exists(cursorDir),
		Docs:      "https://cursor.com/docs/context/mcp",
	}

	// VS Code: top-level "servers" with "type": "stdio"; `code --add-mcp
	// '<json with name>'` adds to the user profile. Source:
	// https://code.visualstudio.com/docs/agents/reference/mcp-configuration
	// The file path <user dir>/Code/User/mcp.json is not stated there (the
	// user dir is from https://code.visualstudio.com/docs/configure/profiles):
	// unverified, so the CLI is preferred.
	vsUser := filepath.Join(app, "Code", "User")
	codeCLI := env.look("code")
	vscode := Client{
		ID: "vscode", Name: "VS Code",
		Path: filepath.Join(vsUser, "mcp.json"), Key: "servers", Typed: true, CLI: codeCLI,
		Installed: codeCLI != "" || env.Exists(vsUser),
		Docs:      "https://code.visualstudio.com/docs/agents/reference/mcp-configuration",
	}

	// Windsurf: docs.windsurf.com now redirects to Devin desktop, whose MCP
	// page names %APPDATA%\devin\mcp_config.json (Windows) and
	// ~/.config/devin/mcp_config.json or $XDG_CONFIG_HOME/devin (macOS,
	// Linux), top-level "mcpServers". Source:
	// https://docs.devin.ai/desktop/cascade/mcp
	// Older Windsurf versions use ~/.codeium/windsurf/mcp_config.json; that
	// path is only in third-party guides now: unverified.
	devinDir := filepath.Join(env.xdg(), "devin")
	if env.GOOS == "windows" {
		devinDir = filepath.Join(env.AppData, "devin")
	}
	legacyDir := filepath.Join(env.Home, ".codeium", "windsurf")
	windsurfDir := legacyDir
	if env.Exists(devinDir) {
		windsurfDir = devinDir
	}
	windsurf := Client{
		ID: "windsurf", Name: "Windsurf",
		Path: filepath.Join(windsurfDir, "mcp_config.json"), Key: "mcpServers",
		Installed: env.Exists(devinDir) || env.Exists(legacyDir),
		Docs:      "https://docs.devin.ai/desktop/cascade/mcp",
	}

	// Gemini CLI: user settings ~/.gemini/settings.json, top-level
	// "mcpServers". Source: https://geminicli.com/docs/tools/mcp-server/
	geminiDir := filepath.Join(env.Home, ".gemini")
	gemini := Client{
		ID: "gemini", Name: "Gemini CLI",
		Path: filepath.Join(geminiDir, "settings.json"), Key: "mcpServers",
		Installed: env.Exists(geminiDir) || env.look("gemini") != "",
		Docs:      "https://geminicli.com/docs/tools/mcp-server/",
	}

	return []Client{desktop, claudeCode, cursor, vscode, windsurf, gemini}
}
