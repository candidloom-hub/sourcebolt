#!/bin/sh
# sourcebolt installer for macOS and Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh
#
# Options (after `sh -s --` when piped, e.g. `... | sh -s -- --uninstall`):
#   --version vX.Y.Z   install this release instead of the latest
#   --dir DIR          install into DIR instead of ~/.local/bin
#   --no-setup         do not offer to run `sourcebolt setup` afterwards
#   --no-modify-path   do not add the install folder to PATH in your shell startup file
#   --uninstall        remove the sourcebolt binary and the PATH lines it added (your sign-in
#                      data is kept)
#   --purge            with --uninstall: also delete your sourcebolt data (asks first)
#   --yes              answer "yes" to the --purge question (for scripts)
#
# Environment variables (same meaning as the options):
#   SOURCEBOLT_VERSION, SOURCEBOLT_INSTALL_DIR, SOURCEBOLT_NO_SETUP=1, SOURCEBOLT_NO_MODIFY_PATH=1
#   SOURCEBOLT_DOWNLOAD_BASE  where release files are fetched from (default: GitHub releases;
#                             must be https://, or file:// for the installer tests)
#
# What it does: downloads the release archive and checksums.txt, checks the SHA-256 of the
# archive (and the cosign signature of checksums.txt when cosign is installed), and copies the
# single `sourcebolt` binary into ~/.local/bin. If that folder is not on your PATH, it appends a
# short block marked "# added by sourcebolt installer" to your shell startup file (~/.zshrc for
# zsh, ~/.bashrc plus ~/.bash_profile or ~/.profile for bash, fish's config.fish for fish,
# ~/.profile otherwise). It needs no root access. Running it again upgrades in place and never
# adds the block twice; --uninstall removes exactly that block.
#
# Everything below runs from main(), called on the last line, so a download cut off halfway
# runs nothing.

set -eu

REPO="candidloom-hub/sourcebolt"
MARK_BEGIN="# >>> added by sourcebolt installer >>>"
MARK_END="# <<< added by sourcebolt installer <<<"
TMP=""
NEWBIN=""
BACKUP=""

say() { printf '%s\n' "$*"; }
step() { printf '  - %s\n' "$*"; }
die() {
	printf '\nsourcebolt was NOT installed: %s\n' "$*" >&2
	exit 1
}
cleanup() {
	if [ -n "$TMP" ] && [ -d "$TMP" ]; then rm -rf "$TMP"; fi
	# A failure halfway through an upgrade: the old binary is still in place, so its backup
	# and any half-copied new binary can go.
	if [ -n "$NEWBIN" ]; then rm -f "$NEWBIN"; fi
	if [ -n "$BACKUP" ] && [ -e "${INSTALL_DIR}/sourcebolt" ]; then rm -f "$BACKUP"; fi
}

usage() {
	cat <<'EOF'
sourcebolt installer for macOS and Linux.

  curl -fsSL https://raw.githubusercontent.com/candidloom-hub/sourcebolt/main/install.sh | sh

Options (after `sh -s --` when piped):
  --version vX.Y.Z   install this release instead of the latest
  --dir DIR          install into DIR instead of ~/.local/bin
  --no-setup         do not offer to run `sourcebolt setup` afterwards
  --no-modify-path   do not add the install folder to PATH in your shell startup file
  --uninstall        remove the sourcebolt binary and the PATH lines it added
  --purge            with --uninstall: also delete your sourcebolt data (asks first)
  --yes              answer "yes" to the --purge question
EOF
}

parse_args() {
	while [ $# -gt 0 ]; do
		case "$1" in
		--version)
			[ $# -ge 2 ] || die "--version needs a value, for example --version v0.2.0"
			VERSION="$2"
			shift 2
			;;
		--version=*)
			VERSION="${1#*=}"
			shift
			;;
		--dir)
			[ $# -ge 2 ] || die "--dir needs a folder"
			INSTALL_DIR="$2"
			shift 2
			;;
		--dir=*)
			INSTALL_DIR="${1#*=}"
			shift
			;;
		--no-setup)
			NO_SETUP=1
			shift
			;;
		--no-modify-path)
			NO_MODIFY_PATH=1
			shift
			;;
		--uninstall)
			ACTION=uninstall
			shift
			;;
		--purge)
			PURGE=1
			shift
			;;
		--yes | -y)
			ASSUME_YES=1
			shift
			;;
		-h | --help)
			usage
			exit 0
			;;
		*) die "unknown option '$1' (try --help)" ;;
		esac
	done
}

# True when a person is at a terminal. With `curl ... | sh` stdin is the script itself, so we
# look for a controlling terminal at /dev/tty instead of testing stdin.
interactive() {
	[ -t 1 ] || return 1
	(: </dev/tty) 2>/dev/null || return 1
	return 0
}

ask_yes() { # ask_yes "question" default(y|n)
	if ! interactive; then return 1; fi
	if [ "$2" = y ]; then hint="[Y/n]"; else hint="[y/N]"; fi
	printf '%s %s ' "$1" "$hint"
	answer=""
	read -r answer </dev/tty || answer=""
	case "$answer" in
	[Yy]*) return 0 ;;
	[Nn]*) return 1 ;;
	"") [ "$2" = y ] ;;
	*) return 1 ;;
	esac
}

config_dir() {
	if [ -n "${NOTEBOOKLM_CONFIG_DIR:-}" ]; then
		printf '%s\n' "$NOTEBOOKLM_CONFIG_DIR"
	elif [ "$(uname -s)" = Darwin ]; then
		printf '%s\n' "${HOME}/Library/Application Support/sourcebolt"
	else
		printf '%s\n' "${XDG_CONFIG_HOME:-${HOME}/.config}/sourcebolt"
	fi
}

# The install folder is written into a shell startup file inside double quotes, so it must be
# an absolute path with no character that the shell would treat specially there.
check_install_dir() {
	nl='
'
	case "$INSTALL_DIR" in
	"" | / | "$HOME" | "$HOME"/) die "'${INSTALL_DIR}' cannot be used as the install folder. Pick a folder of its own, for example ~/.local/bin." ;;
	/*) ;;
	*) die "'${INSTALL_DIR}' cannot be used as the install folder: it must be a full path starting with /." ;;
	esac
	case "$INSTALL_DIR" in
	*'"'* | *'$'* | *'`'* | *\\* | *"$nl"*) die "'${INSTALL_DIR}' cannot be used as the install folder: it contains a quote, \$, backtick, backslash or line break." ;;
	esac
	INSTALL_DIR="${INSTALL_DIR%/}"
}

# Refuses to recursively delete a folder that is clearly not sourcebolt's own: the home folder,
# any folder above it, the shared config folders, or a folder directly under /. Paths are
# compared after resolving symlinks, "..", and doubled or trailing slashes.
check_purge_dir() {
	case "$1" in
	/*) ;;
	*) die "refusing to delete '${1}': it is not a full path. Check NOTEBOOKLM_CONFIG_DIR." ;;
	esac
	d="$(cd "$1" 2>/dev/null && pwd -P)" || die "refusing to delete '${1}': it could not be resolved. Check NOTEBOOKLM_CONFIG_DIR."
	h="$(cd "$HOME" 2>/dev/null && pwd -P)" || h="${HOME%/}"
	case "${d#/}" in
	*/*) ;;
	*) die "refusing to delete '${1}': it is a system folder. Check NOTEBOOKLM_CONFIG_DIR." ;;
	esac
	case "$h/" in
	"$d"/*) die "refusing to delete '${1}': it is your home folder or a folder above it. Check NOTEBOOKLM_CONFIG_DIR." ;;
	esac
	for shared in "${XDG_CONFIG_HOME:-${HOME}/.config}" "${HOME}/.config" "${HOME}/Library/Application Support" "${HOME}/Library"; do
		s="$(cd "$shared" 2>/dev/null && pwd -P)" || continue
		[ "$d" = "$s" ] && die "refusing to delete '${1}': it is a shared settings folder, not a sourcebolt data folder. Check NOTEBOOKLM_CONFIG_DIR."
	done
	return 0
}

check_version() {
	case "$VERSION" in
	v*) ;;
	*) VERSION="v${VERSION}" ;;
	esac
	printf '%s\n' "$VERSION" | grep -Eqx 'v[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9_.]+)?' ||
		die "'${VERSION}' is not a valid version. Use a release tag such as v0.2.0."
}

check_download_base() {
	case "$DOWNLOAD_BASE" in
	https://* | file://*) ;;
	*) die "SOURCEBOLT_DOWNLOAD_BASE must start with https:// (got '${DOWNLOAD_BASE}'). Only HTTPS downloads are allowed." ;;
	esac
}

on_path() {
	case ":${PATH}:" in
	*":$1:"* | *":$1/:"*) return 0 ;;
	*) return 1 ;;
	esac
}

# Every startup file the installer may have written to.
rc_candidates() {
	printf '%s\n' "${ZDOTDIR:-${HOME}}/.zshrc" "${HOME}/.bashrc" "${HOME}/.bash_profile" "${HOME}/.profile" \
		"${XDG_CONFIG_HOME:-${HOME}/.config}/fish/config.fish"
}

# The startup files to write for the user's login shell ($SHELL).
rc_targets() {
	case "$(basename "${SHELL:-sh}")" in
	zsh) printf '%s\n' "${ZDOTDIR:-${HOME}}/.zshrc" ;;
	bash)
		printf '%s\n' "${HOME}/.bashrc"
		# Login shells (macOS Terminal, ssh) read ~/.bash_profile if it exists, else ~/.profile.
		if [ -f "${HOME}/.bash_profile" ]; then printf '%s\n' "${HOME}/.bash_profile"; else printf '%s\n' "${HOME}/.profile"; fi
		;;
	fish) printf '%s\n' "${XDG_CONFIG_HOME:-${HOME}/.config}/fish/config.fish" ;;
	*) printf '%s\n' "${HOME}/.profile" ;;
	esac
}

# Writes $HOME instead of the expanded home folder when possible, so the line stays readable.
path_for_rc() {
	case "$1" in
	"${HOME}"/*) printf '%s\n' "\$HOME/${1#"${HOME}"/}" ;;
	*) printf '%s\n' "$1" ;;
	esac
}

add_path_block() { # add_path_block <file>
	f="$1"
	if [ -f "$f" ] && grep -qxF "$MARK_BEGIN" "$f"; then
		step "PATH already set up in ${f}"
		return 0
	fi
	mkdir -p "$(dirname "$f")"
	d="$(path_for_rc "$INSTALL_DIR")"
	# Start on a fresh line if the file does not end with a newline.
	lead=""
	if [ -s "$f" ] && [ "$(tail -c 1 "$f" | od -An -c | tr -d ' ')" != '\n' ]; then lead="
"
	fi
	# $PATH below is meant literally: the user's shell expands it at startup.
	# shellcheck disable=SC2016
	case "$f" in
	*.fish) line="$(printf 'contains "%s" $PATH; or set -gx PATH "%s" $PATH' "$d" "$d")" ;;
	*) line="$(printf 'case ":${PATH}:" in *":%s:"*) ;; *) export PATH="%s:${PATH}" ;; esac' "$d" "$d")" ;;
	esac
	printf '%s%s\n%s\n%s\n' "$lead" "$MARK_BEGIN" "$line" "$MARK_END" >>"$f"
	step "added ${INSTALL_DIR} to PATH in ${f}"
}

remove_path_blocks() {
	rc_candidates | while IFS= read -r f; do
		if [ ! -f "$f" ] || ! grep -qxF "$MARK_BEGIN" "$f"; then continue; fi
		if ! grep -qxF "$MARK_END" "$f"; then
			step "left ${f} unchanged: it has the sourcebolt start line but no end marker. Remove the lines after '${MARK_BEGIN}' by hand."
			continue
		fi
		# Write the new content next to the file, then move it into place, so the file is never
		# left half-written. cp -p first keeps the file's permissions on the new copy.
		tmp="$(mktemp "${f}.sourcebolt.XXXXXX")" || {
			step "could not create a temporary file next to ${f}; left it unchanged"
			continue
		}
		if ! cp -p "$f" "$tmp"; then
			rm -f "$tmp"
			step "could not copy ${f}; left it unchanged"
			continue
		fi
		# Lines inside a block are held back and dropped only when its end marker comes; a start
		# marker that is never closed gets its lines printed back at the end of the file.
		if ! awk -v b="$MARK_BEGIN" -v e="$MARK_END" '
			!skip && $0 == b { skip = 1; held = $0; next }
			skip && $0 == e { skip = 0; held = ""; next }
			skip { held = held "\n" $0; next }
			{ print }
			END { if (skip) print held }' "$f" >"$tmp"; then
			rm -f "$tmp"
			step "could not rewrite ${f}; left it unchanged"
			continue
		fi
		if [ -L "$f" ]; then
			# Keep the symlink: write through it.
			cat "$tmp" >"$f"
			rm -f "$tmp"
		else
			mv -f "$tmp" "$f"
		fi
		step "removed the sourcebolt PATH lines from ${f}"
	done
}

uninstall() {
	say "Removing sourcebolt"
	bin="${INSTALL_DIR}/sourcebolt"
	if [ -e "$bin" ]; then
		rm -f "$bin"
		step "removed ${bin}"
	else
		step "no sourcebolt binary in ${INSTALL_DIR} (nothing to remove)"
	fi
	# Leftovers of an upgrade that was cut off.
	for left in "${INSTALL_DIR}"/.sourcebolt.old.* "${INSTALL_DIR}"/.sourcebolt.new.*; do
		if [ -f "$left" ] && [ ! -L "$left" ]; then rm -f "$left"; fi
	done
	data="$(config_dir)"
	if [ "$PURGE" = 1 ]; then
		if [ ! -d "$data" ]; then
			step "no sourcebolt data at ${data}"
		else
			check_purge_dir "$data"
			if [ "$ASSUME_YES" = 1 ] || ask_yes "Also delete your sourcebolt sign-in data at ${data}? This signs you out for good." n; then
				rm -rf "$data"
				step "deleted ${data}"
			else
				step "kept your sourcebolt data at ${data} (re-run with --purge --yes to delete it)"
			fi
		fi
	elif [ -d "$data" ]; then
		step "kept your sourcebolt data at ${data} (add --purge to delete it)"
	fi
	if [ "$NO_MODIFY_PATH" != 1 ]; then remove_path_blocks; fi
	say "Done. Open a new terminal for the PATH change to take effect."
}

setup_tools() {
	if command -v curl >/dev/null 2>&1; then
		# --proto limits the first request, --proto-redir every redirect: never plain HTTP.
		fetch() { curl -fsSL --retry 3 --proto '=https,file' --proto-redir '=https' -o "$2" "$1"; }
		fetch_stdout() { curl -fsSL --retry 3 --proto '=https' --proto-redir '=https' "$1"; }
	elif command -v wget >/dev/null 2>&1; then
		# wget cannot refuse an HTTPS-to-HTTP redirect; the URLs it is given are checked to be
		# https:// above, and GitHub never redirects to plain HTTP.
		step "curl not found: using wget, which cannot refuse a redirect to plain HTTP (install curl for the strictest download)"
		fetch() {
			case "$1" in https://*) ;; *) return 1 ;; esac
			wget -q -O "$2" "$1"
		}
		fetch_stdout() {
			case "$1" in https://*) ;; *) return 1 ;; esac
			wget -q -O - "$1"
		}
	else
		die "this installer needs curl or wget. Install one of them and try again."
	fi

	if command -v sha256sum >/dev/null 2>&1; then
		sha256() { sha256sum "$1" | awk '{print $1}'; }
	elif command -v shasum >/dev/null 2>&1; then
		sha256() { shasum -a 256 "$1" | awk '{print $1}'; }
	elif command -v openssl >/dev/null 2>&1; then
		sha256() { openssl dgst -sha256 "$1" | awk '{print $NF}'; }
	else
		die "could not find sha256sum, shasum or openssl to check the download."
	fi

	command -v tar >/dev/null 2>&1 || die "this installer needs 'tar'."
}

detect_platform() {
	case "$(uname -s)" in
	Linux) OS=linux ;;
	Darwin) OS=darwin ;;
	MINGW* | MSYS* | CYGWIN*) die "on Windows, use the PowerShell installer: irm https://raw.githubusercontent.com/${REPO}/main/install.ps1 | iex" ;;
	*) die "unsupported system '$(uname -s)'. See https://github.com/${REPO}/blob/main/docs/install.md for other ways to install." ;;
	esac
	case "$(uname -m)" in
	x86_64 | amd64) ARCH=amd64 ;;
	arm64 | aarch64) ARCH=arm64 ;;
	*) die "unsupported processor '$(uname -m)'. Builds exist for x86_64 and arm64 only." ;;
	esac
	# Apple Silicon running this shell under Rosetta reports x86_64; prefer the native build.
	if [ "$OS" = darwin ] && [ "$ARCH" = amd64 ] && [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = 1 ]; then
		ARCH=arm64
	fi
}

latest_version() {
	VERSION="$(fetch_stdout "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null |
		sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)" || VERSION=""
	if [ -z "$VERSION" ] && command -v curl >/dev/null 2>&1; then
		# API rate-limited? The /releases/latest page redirects to /releases/tag/<tag>.
		VERSION="$(curl -fsSLI --proto '=https' --proto-redir '=https' -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" 2>/dev/null | sed -n 's#.*/tag/##p')" || VERSION=""
	fi
	[ -n "$VERSION" ] || die "could not find the latest release. Check your internet connection, or set SOURCEBOLT_VERSION=v0.2.0."
}

download_and_verify() {
	step "downloading ${ARCHIVE}"
	fetch "${BASE}/${ARCHIVE}" "${TMP}/${ARCHIVE}" || die "could not download ${BASE}/${ARCHIVE}"
	fetch "${BASE}/checksums.txt" "${TMP}/checksums.txt" || die "could not download ${BASE}/checksums.txt"

	if command -v cosign >/dev/null 2>&1; then
		step "cosign found: checking the signature of checksums.txt"
		fetch "${BASE}/checksums.txt.sigstore.json" "${TMP}/checksums.txt.sigstore.json" ||
			die "cosign is installed but the signature file could not be downloaded."
		cosign verify-blob "${TMP}/checksums.txt" \
			--bundle "${TMP}/checksums.txt.sigstore.json" \
			--certificate-identity-regexp "^https://github.com/${REPO}/\.github/workflows/release\.yml@refs/tags/v" \
			--certificate-oidc-issuer "https://token.actions.githubusercontent.com" >/dev/null 2>&1 ||
			die "the signature of checksums.txt did not verify. The download may have been tampered with."
		step "signature verified (cosign)"
	else
		step "cosign not installed: signature check skipped. The SHA-256 check below is integrity only: it proves the file matches checksums.txt, not who published it."
	fi

	expected="$(awk -v f="$ARCHIVE" '$2 == f || $2 == "*" f {print $1}' "${TMP}/checksums.txt" | head -n 1)"
	[ -n "$expected" ] || die "checksums.txt has no entry for ${ARCHIVE}."
	actual="$(sha256 "${TMP}/${ARCHIVE}")"
	expected_lc="$(printf '%s' "$expected" | tr 'A-F' 'a-f')"
	actual_lc="$(printf '%s' "$actual" | tr 'A-F' 'a-f')"
	if [ "$expected_lc" != "$actual_lc" ]; then
		die "checksum mismatch for ${ARCHIVE} (expected ${expected_lc}, got ${actual_lc}). The download may be corrupt or tampered with; nothing was changed."
	fi
	step "SHA-256 checksum verified"
}

install_binary() {
	mkdir -p "${TMP}/x"
	tar -xzf "${TMP}/${ARCHIVE}" -C "${TMP}/x" sourcebolt || die "could not unpack ${ARCHIVE}."
	if [ -L "${TMP}/x/sourcebolt" ] || [ ! -f "${TMP}/x/sourcebolt" ]; then
		die "${ARCHIVE} does not contain a sourcebolt binary."
	fi
	chmod 0755 "${TMP}/x/sourcebolt"
	if [ "$OS" = darwin ] && command -v xattr >/dev/null 2>&1; then
		xattr -d com.apple.quarantine "${TMP}/x/sourcebolt" 2>/dev/null || true
	fi

	mkdir -p "$INSTALL_DIR" || die "could not create ${INSTALL_DIR}."
	BIN="${INSTALL_DIR}/sourcebolt"
	# Temporary files next to the target get unpredictable names from mktemp (a shared folder
	# could otherwise hold a planted symlink), and cleanup removes whatever is left of them.
	if [ -e "$BIN" ]; then
		# Keep the old binary until the new one has run, so a broken upgrade can be undone.
		BACKUP="$(mktemp "${INSTALL_DIR}/.sourcebolt.old.XXXXXX")" || die "could not write to ${INSTALL_DIR}."
		cp -p "$BIN" "$BACKUP" || die "could not back up ${BIN}."
	fi
	# Copy next to the target, then rename: atomic, and safe while an old copy is running.
	NEWBIN="$(mktemp "${INSTALL_DIR}/.sourcebolt.new.XXXXXX")" || die "could not write to ${INSTALL_DIR}."
	cp "${TMP}/x/sourcebolt" "$NEWBIN" || die "could not write to ${INSTALL_DIR}."
	chmod 0755 "$NEWBIN"
	mv -f "$NEWBIN" "$BIN" || die "could not replace ${BIN}."
	NEWBIN=""
	backup="$BACKUP"

	if ! installed_version="$("$BIN" --version 2>/dev/null)"; then
		if [ -n "$backup" ]; then
			mv -f "$backup" "$BIN"
			BACKUP=""
			die "the new binary did not run (${BIN} --version); restored the previous version."
		fi
		rm -f "$BIN"
		die "the installed binary did not run: ${BIN} --version"
	fi
	if [ -n "$backup" ]; then
		rm -f "$backup"
		BACKUP=""
		step "upgraded ${BIN}"
	else
		step "installed ${BIN}"
	fi
	say ""
	say "Installed: ${installed_version}"
}

setup_path() {
	on_path "$INSTALL_DIR" && return 0
	say ""
	if [ "$NO_MODIFY_PATH" = 1 ]; then
		say "Note: ${INSTALL_DIR} is not on your PATH, so typing 'sourcebolt' will not work yet."
		say "Add this line to your shell startup file (~/.zshrc, ~/.bashrc or ~/.profile), then open a new terminal:"
		say ""
		say "    export PATH=\"${INSTALL_DIR}:\$PATH\""
	else
		say "Adding ${INSTALL_DIR} to your PATH:"
		rc_targets | while IFS= read -r f; do add_path_block "$f"; done
		say "Open a new terminal so that typing 'sourcebolt' works."
	fi
	say "Until then, run it as: ${BIN}"
}

next_step() {
	say ""
	if "$BIN" setup --help >/dev/null 2>&1; then
		if [ "$NO_SETUP" = 1 ]; then
			say "Next step: run 'sourcebolt setup' to sign in and connect your AI apps."
		elif ask_yes "Run 'sourcebolt setup' now to sign in and connect your AI apps?" y; then
			"$BIN" setup </dev/tty || say "Setup did not finish. You can run 'sourcebolt setup' again at any time."
		else
			say "Next step: run 'sourcebolt setup' to sign in and connect your AI apps."
		fi
	else
		say "Next step: sign in to NotebookLM. run: sourcebolt login"
		say "Guide: https://github.com/${REPO}/blob/main/docs/auth.md"
	fi
	say "To uninstall later: curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sh -s -- --uninstall"
}

main() {
	DOWNLOAD_BASE="${SOURCEBOLT_DOWNLOAD_BASE:-https://github.com/${REPO}/releases/download}"
	VERSION="${SOURCEBOLT_VERSION:-}"
	INSTALL_DIR="${SOURCEBOLT_INSTALL_DIR:-${HOME}/.local/bin}"
	NO_SETUP="${SOURCEBOLT_NO_SETUP:-0}"
	NO_MODIFY_PATH="${SOURCEBOLT_NO_MODIFY_PATH:-0}"
	ACTION=install
	PURGE=0
	ASSUME_YES=0

	trap cleanup EXIT
	trap 'exit 130' INT TERM

	parse_args "$@"
	check_install_dir

	if [ "$ACTION" = uninstall ]; then
		uninstall
		exit 0
	fi

	check_download_base
	if [ -n "$VERSION" ]; then check_version; fi
	setup_tools
	detect_platform
	if [ -z "$VERSION" ]; then
		latest_version
		check_version
	fi

	NUM="${VERSION#v}"
	ARCHIVE="sourcebolt_${NUM}_${OS}_${ARCH}.tar.gz"
	BASE="${DOWNLOAD_BASE%/}/${VERSION}"

	say "Installing sourcebolt ${VERSION} for ${OS}/${ARCH}"
	TMP="$(mktemp -d 2>/dev/null || mktemp -d -t sourcebolt)"

	download_and_verify
	install_binary
	setup_path
	next_step
}

main "$@"
