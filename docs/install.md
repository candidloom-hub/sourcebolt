# Install sourcebolt

sourcebolt is one small program. Pick **one** of the ways below. If you are not sure, use the one-line installer for your system.

| Way | Good for | Status |
|---|---|---|
| [One-line installer](#one-line-installer-recommended) | Most people (Windows, macOS, Linux) | Available |
| [npm (`npx`)](#npm-npx) | People who already have Node.js | Available |
| [Homebrew](#homebrew-macos-and-linux) | macOS and Linux users of Homebrew | Available |
| [Scoop](#scoop-windows) | Windows users of Scoop | Available |
| [winget](#winget-windows) | Windows users of winget | Submitted, in review |
| [Docker](#docker) | Running it in a container | Available |
| [Manual download](#manual-download) | Checking every step yourself | Available |
| [`go install`](#go-install) | Go developers | Available |

After installing, sign in to NotebookLM (see [auth.md](auth.md)) and connect your AI app (see the [README](../README.md#connect-your-ai-app)).

## One-line installer (recommended)

### Windows

Open **PowerShell** (press Start, type `PowerShell`, press Enter) and paste:

```powershell
irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 | iex
```

You should see something like:

```text
Installing sourcebolt v0.3.1 for windows/amd64
  - downloading sourcebolt_0.3.1_windows_amd64.zip
  - cosign not installed: signature check skipped. The SHA-256 check below is integrity only: it proves the file matches checksums.txt, not who published it.
  - SHA-256 checksum verified
  - installed C:\Users\you\AppData\Local\Programs\sourcebolt\sourcebolt.exe

Installed: sourcebolt 0.3.1
  - added C:\Users\you\AppData\Local\Programs\sourcebolt to your user PATH (open a new terminal to use 'sourcebolt')

Next step: sign in to NotebookLM. run: sourcebolt login
```

Then **open a new PowerShell window** and check it works:

```powershell
sourcebolt --version
```

```text
sourcebolt 0.3.1
```

### macOS and Linux

Open **Terminal** and paste:

```bash
curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh
```

You should see something like this (macOS with zsh):

```text
Installing sourcebolt v0.3.1 for darwin/arm64
  - downloading sourcebolt_0.3.1_darwin_arm64.tar.gz
  - cosign not installed: signature check skipped. The SHA-256 check below is integrity only: it proves the file matches checksums.txt, not who published it.
  - SHA-256 checksum verified
  - installed /Users/you/.local/bin/sourcebolt

Installed: sourcebolt 0.3.1

Adding /Users/you/.local/bin to your PATH:
  - added /Users/you/.local/bin to PATH in /Users/you/.zshrc
Open a new terminal so that typing 'sourcebolt' works.
Until then, run it as: /Users/you/.local/bin/sourcebolt

Next step: sign in to NotebookLM. run: sourcebolt login
```

Then **open a new terminal window** and run `sourcebolt --version`. It prints `sourcebolt 0.3.1`.

### What the installer does

1. Works out your system (Windows, macOS or Linux) and processor (x64 or ARM64).
2. Asks GitHub for the latest release. To pick a version, set `SOURCEBOLT_VERSION=v0.2.0` first.
3. Downloads the release archive and its `checksums.txt` from the [GitHub release](https://github.com/candidloom-hub/sourcebolt/releases).
4. Checks the archive's SHA-256 checksum against `checksums.txt`. If they differ, it stops and changes nothing. If [cosign](https://docs.sigstore.dev/cosign/system_config/installation/) is installed, it also checks that `checksums.txt` was signed by this project's release workflow, and says whether it did.
5. Copies the program to a folder in your user account. It never asks for an administrator password.
   - Windows: `%LOCALAPPDATA%\Programs\sourcebolt\sourcebolt.exe`
   - macOS and Linux: `~/.local/bin/sourcebolt`
6. Makes `sourcebolt` work in new terminals:
   - Windows: adds that folder to your **user** `PATH`.
   - macOS and Linux: only if `~/.local/bin` is not on your `PATH` yet, it adds three lines to your shell startup file, between `# >>> added by sourcebolt installer >>>` and `# <<< added by sourcebolt installer <<<`. The file is `~/.zshrc` for zsh (the macOS default), `~/.bashrc` plus `~/.bash_profile` or `~/.profile` for bash, `~/.config/fish/config.fish` for fish, and `~/.profile` for other shells.
7. Prints `Installed: sourcebolt <version>`, then tells you the next step. When a newer sourcebolt with a `setup` command is installed, it offers to run `sourcebolt setup` for you (only when you are at a terminal; set `SOURCEBOLT_NO_SETUP=1` to skip the question).

Running the installer again upgrades sourcebolt in place. It never adds the `PATH` lines twice.

Options:

| Windows (`install.ps1`) | macOS and Linux (`install.sh`) | What it does |
|---|---|---|
| `-Version v0.2.0` | `--version v0.2.0` | Install that version instead of the latest |
| `-InstallDir <folder>` | `--dir <folder>` | Install somewhere else |
| `-NoModifyPath` | `--no-modify-path` | Do not touch `PATH` or your shell startup file |
| `-NoSetup` | `--no-setup` | Do not offer to run `sourcebolt setup` |
| `-Uninstall` | `--uninstall` | Remove sourcebolt (see below) |
| `-Purge` (with `-Uninstall`) | `--purge` (with `--uninstall`) | Also delete your sourcebolt data, after asking |

To pass options, use the forms shown under "Undo it" below: `& ([scriptblock]::Create((irm <url>))) -Option` on Windows, and `curl -fsSL <url> | sh -s -- --option` on macOS and Linux.

### Undo it

**Windows** (PowerShell):

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1))) -Uninstall
```

**macOS and Linux**:

```bash
curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh -s -- --uninstall
```

`install.ps1 -Uninstall` removes `sourcebolt.exe` and its user `PATH` entry. `install.sh --uninstall` removes `~/.local/bin/sourcebolt` and exactly the marked lines it added to your shell startup files. Both keep your sign-in data, so a later reinstall still works without signing in again.

To also delete your sign-in data (this signs you out for good), add `-Purge` / `--purge`. The installer asks before deleting anything:

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1))) -Uninstall -Purge
```

```bash
curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh -s -- --uninstall --purge
```

The data folder is `%APPDATA%\sourcebolt` on Windows, `~/Library/Application Support/sourcebolt` on macOS and `~/.config/sourcebolt` on Linux.

### Prefer to read it first?

Download the script, read it, then run it:

```bash
curl -fsSLO https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh
less install.sh
sh install.sh
```

```powershell
irm https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.ps1 -OutFile install.ps1
notepad install.ps1
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

## npm (`npx`)

Needs [Node.js](https://nodejs.org/) 18 or newer. npm downloads the right program for your system the first time.

```bash
npx -y sourcebolt --version
```

```text
sourcebolt 0.3.1
```

In an AI app's MCP settings, use `npx` as the command:

```json
{ "command": "npx", "args": ["-y", "sourcebolt", "serve"] }
```

## Homebrew (macOS and Linux)

Every release updates the cask in [candidloom-hub/homebrew-tap](https://github.com/candidloom-hub/homebrew-tap).

```bash
brew install --cask candidloom-hub/tap/sourcebolt
sourcebolt --version
```

Undo: `brew uninstall --cask sourcebolt`.

## Scoop (Windows)

Every release updates the manifest in [candidloom-hub/scoop-bucket](https://github.com/candidloom-hub/scoop-bucket).

```powershell
scoop bucket add candidloom-hub https://github.com/candidloom-hub/scoop-bucket
scoop install candidloom-hub/sourcebolt
sourcebolt --version
```

Undo: `scoop uninstall sourcebolt`.

## winget (Windows)

Submitted to the winget catalog and waiting for review; the command below works once the catalog accepts it.

```powershell
winget install candidloom-hub.sourcebolt
```

Open a new terminal, then run `sourcebolt --version`. Undo: `winget uninstall candidloom-hub.sourcebolt`.

## Docker

```bash
docker run -i --rm -v sourcebolt-config:/config ghcr.io/candidloom-hub/sourcebolt:0.3.1 --version
```

```text
sourcebolt 0.3.1
```

Images are tagged by version only (there is no `latest` tag). The `0.2.0-browser` tag adds a browser for the fallback path. Signing in inside a container: [auth.md](auth.md#docker). Undo: `docker image rm ghcr.io/candidloom-hub/sourcebolt:0.3.1` and `docker volume rm sourcebolt-config` (the volume holds your sign-in data).

## Manual download

The [README](../README.md#release-binary) has step-by-step commands for downloading a release archive yourself, and [security.md](security.md#verify-a-download) shows how to check its checksum and signature.

## `go install`

Needs Go 1.27 or newer.

```bash
go install github.com/candidloom-hub/sourcebolt/cmd/sourcebolt@latest
sourcebolt --version
```

The program goes to `$(go env GOPATH)/bin`. A binary built this way prints `sourcebolt dev`, because the version number is only stamped into release builds. Undo: delete `sourcebolt` (or `sourcebolt.exe`) from that folder.
