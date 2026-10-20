#!/bin/sh
# Offline tests for install.sh. They build a fake release (a shell script standing in for the
# binary) and point the installer at it with SOURCEBOLT_DOWNLOAD_BASE=file://..., so no network
# is used. `uname` is shimmed to Linux/x86_64 so the same tests run on Linux, macOS and Git Bash.
#
#   sh scripts/test-installers.sh
#
# Cases: good install, idempotent upgrade, tampered checksum (must abort and leave the old binary
# untouched), archive missing from checksums.txt (must abort), PATH block in the right startup
# file for zsh, bash and fish (added once, works when sourced, skipped with --no-modify-path),
# uninstall removes exactly that block and keeps data, --purge --yes deletes data.
# HOME is a temporary folder, so no real startup file is touched.

set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALLER="${ROOT}/install.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass=0
fail=0
ok() {
	pass=$((pass + 1))
	printf 'ok   %s\n' "$1"
}
bad() {
	fail=$((fail + 1))
	printf 'FAIL %s\n' "$1"
	if [ -f "$WORK/out" ]; then sed 's/^/     | /' "$WORK/out"; fi
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
}

# uname shim
mkdir -p "$WORK/shim"
cat >"$WORK/shim/uname" <<'EOF'
#!/bin/sh
case "${1:-}" in
-m) echo x86_64 ;;
*) echo Linux ;;
esac
EOF
chmod +x "$WORK/shim/uname"

# make_release <version> <printed version>
make_release() {
	v="$1"
	d="$WORK/rel/v$v"
	mkdir -p "$d" "$WORK/pkg"
	# shellcheck disable=SC2016 # the fake binary's own script text
	printf '#!/bin/sh\nif [ "${1:-}" = --version ]; then echo "sourcebolt %s"; exit 0; fi\nexit 2\n' "$2" >"$WORK/pkg/sourcebolt"
	chmod +x "$WORK/pkg/sourcebolt"
	(cd "$WORK/pkg" && tar -czf "$d/sourcebolt_${v}_linux_amd64.tar.gz" sourcebolt)
	printf '%s  %s\n' "$(sha256 "$d/sourcebolt_${v}_linux_amd64.tar.gz")" "sourcebolt_${v}_linux_amd64.tar.gz" >"$d/checksums.txt"
}

# Git Bash ships a native Windows curl, which needs a Windows path in file:// URLs.
if command -v cygpath >/dev/null 2>&1; then BASE_URL="file:///$(cygpath -m "$WORK/rel")"; else BASE_URL="file://$WORK/rel"; fi

H="$WORK/home"
BINDIR="$H/.local/bin"
TEST_SHELL=/bin/sh

run_installer() { # extra args...
	env PATH="$WORK/shim:$PATH" HOME="$H" SHELL="$TEST_SHELL" ZDOTDIR="$H" XDG_CONFIG_HOME="$H/.config" \
		SOURCEBOLT_DOWNLOAD_BASE="$BASE_URL" \
		SOURCEBOLT_INSTALL_DIR="$BINDIR" \
		NOTEBOOKLM_CONFIG_DIR="$WORK/config" \
		SOURCEBOLT_NO_SETUP=1 \
		sh "$INSTALLER" "$@" >"$WORK/out" 2>&1
}

count_blocks() { # count_blocks <file>
	if [ -f "$1" ]; then grep -c '^# >>> added by sourcebolt installer >>>$' "$1" || true; else echo 0; fi
}

mkdir -p "$H"
make_release 1.0.0 1.0.0
make_release 1.1.0 1.1.0

# 1. good install
if run_installer --version v1.0.0 && [ "$("$BINDIR/sourcebolt" --version)" = "sourcebolt 1.0.0" ] &&
	grep -q 'SHA-256 checksum verified' "$WORK/out" && grep -q '^Installed: sourcebolt 1.0.0$' "$WORK/out"; then
	ok "installs and verifies the checksum"
else bad "installs and verifies the checksum"; fi

# 2. upgrade (and re-run of the same version) in place
if run_installer --version 1.1.0 && [ "$("$BINDIR/sourcebolt" --version)" = "sourcebolt 1.1.0" ] &&
	grep -q 'upgraded' "$WORK/out" && run_installer --version v1.1.0; then
	ok "upgrades in place and re-runs cleanly"
else bad "upgrades in place and re-runs cleanly"; fi

# 3. tampered checksum: abort, keep the installed binary as it was
cp "$WORK/rel/v1.0.0/checksums.txt" "$WORK/good-checksums"
awk '{c = substr($1, 1, 1); n = (c == "a") ? "b" : "a"; print n substr($1, 2) "  " $2}' "$WORK/good-checksums" >"$WORK/rel/v1.0.0/checksums.txt"
if cmp -s "$WORK/good-checksums" "$WORK/rel/v1.0.0/checksums.txt"; then
	bad "tamper fixture differs from the real checksum"
elif run_installer --version v1.0.0; then
	bad "aborts on a tampered checksums.txt (installer exited 0)"
elif grep -q 'checksum mismatch' "$WORK/out" && [ "$("$BINDIR/sourcebolt" --version)" = "sourcebolt 1.1.0" ]; then
	ok "aborts on a tampered checksums.txt and leaves the old binary"
else bad "aborts on a tampered checksums.txt and leaves the old binary"; fi

# 4. tampered archive (checksums.txt intact)
cp "$WORK/good-checksums" "$WORK/rel/v1.0.0/checksums.txt"
cp "$WORK/rel/v1.0.0/sourcebolt_1.0.0_linux_amd64.tar.gz" "$WORK/good.tgz"
printf 'x' >>"$WORK/rel/v1.0.0/sourcebolt_1.0.0_linux_amd64.tar.gz"
if run_installer --version v1.0.0; then
	bad "aborts on a tampered archive (installer exited 0)"
elif grep -q 'checksum mismatch' "$WORK/out"; then
	ok "aborts on a tampered archive"
else bad "aborts on a tampered archive"; fi
cp "$WORK/good.tgz" "$WORK/rel/v1.0.0/sourcebolt_1.0.0_linux_amd64.tar.gz"

# 5. archive missing from checksums.txt
: >"$WORK/rel/v1.0.0/checksums.txt"
if run_installer --version v1.0.0; then
	bad "aborts when checksums.txt has no entry (installer exited 0)"
elif grep -q 'no entry' "$WORK/out"; then
	ok "aborts when checksums.txt has no entry"
else bad "aborts when checksums.txt has no entry"; fi
cp "$WORK/good-checksums" "$WORK/rel/v1.0.0/checksums.txt"

# 6. PATH: plain sh -> ~/.profile, once, and it works when sourced
# shellcheck disable=SC2016 # $HOME and $PATH are expanded by the child shell / kept literal on purpose
if [ "$(count_blocks "$H/.profile")" = 1 ] &&
	[ "$(env PATH=/usr/bin:/bin HOME="$H" sh -c '. "$HOME/.profile"; printf %s "$PATH"' | tr ':' '\n' | grep -cxF "$BINDIR")" = 1 ] &&
	[ "$(env PATH="$BINDIR:/usr/bin:/bin" HOME="$H" sh -c '. "$HOME/.profile"; printf %s "$PATH"' | tr ':' '\n' | grep -cxF "$BINDIR")" = 1 ] &&
	grep -qF '$HOME/.local/bin' "$H/.profile"; then
	ok "adds one PATH block to ~/.profile that works when sourced"
else bad "adds one PATH block to ~/.profile that works when sourced"; fi

# 7. zsh -> ~/.zshrc (existing content kept, file without a trailing newline)
printf 'alias ll="ls -l"' >"$H/.zshrc"
TEST_SHELL=/bin/zsh
if run_installer --version v1.1.0 && run_installer --version v1.1.0 && [ "$(count_blocks "$H/.zshrc")" = 1 ] &&
	head -n 1 "$H/.zshrc" | grep -qx 'alias ll="ls -l"' && grep -q 'open a new terminal\|Open a new terminal' "$WORK/out"; then
	ok "zsh: adds one block to ~/.zshrc and keeps existing lines"
else bad "zsh: adds one block to ~/.zshrc and keeps existing lines"; fi

# 8. bash -> ~/.bashrc and ~/.profile (no ~/.bash_profile)
TEST_SHELL=/bin/bash
if run_installer --version v1.1.0 && [ "$(count_blocks "$H/.bashrc")" = 1 ] && [ "$(count_blocks "$H/.profile")" = 1 ] &&
	[ ! -e "$H/.bash_profile" ]; then
	ok "bash: adds one block to ~/.bashrc and ~/.profile"
else bad "bash: adds one block to ~/.bashrc and ~/.profile"; fi

# 9. fish -> config.fish
TEST_SHELL=/usr/bin/fish
if run_installer --version v1.1.0 && [ "$(count_blocks "$H/.config/fish/config.fish")" = 1 ] &&
	grep -q 'set -gx PATH' "$H/.config/fish/config.fish"; then
	ok "fish: adds one block to config.fish"
else bad "fish: adds one block to config.fish"; fi

# 10. --no-modify-path writes nothing
mv "$H" "$WORK/home-saved"
mkdir -p "$H"
if run_installer --version v1.1.0 --no-modify-path && [ -z "$(find "$H" -mindepth 1 ! -path "$H/.local" ! -path "$H/.local/*")" ] && grep -q 'export PATH=' "$WORK/out"; then
	ok "--no-modify-path leaves startup files alone"
else bad "--no-modify-path leaves startup files alone"; fi
rm -rf "$H"
mv "$WORK/home-saved" "$H"

# 11. uninstall removes the binary and exactly the PATH blocks, and keeps the data folder
mkdir -p "$WORK/config"
echo keep >"$WORK/config/profile"
if run_installer --uninstall && [ ! -e "$BINDIR/sourcebolt" ] && [ -f "$WORK/config/profile" ] &&
	[ "$(count_blocks "$H/.zshrc")$(count_blocks "$H/.bashrc")$(count_blocks "$H/.profile")$(count_blocks "$H/.config/fish/config.fish")" = 0000 ] &&
	! grep -q 'sourcebolt' "$H/.zshrc" "$H/.bashrc" "$H/.profile" "$H/.config/fish/config.fish" &&
	grep -qx 'alias ll="ls -l"' "$H/.zshrc"; then
	ok "uninstall removes the binary and the PATH blocks, keeps other lines and the data"
else bad "uninstall removes the binary and the PATH blocks, keeps other lines and the data"; fi

# 12. --purge without a terminal and without --yes keeps the data
if run_installer --uninstall --purge </dev/null && [ -f "$WORK/config/profile" ]; then
	ok "--purge without confirmation keeps the data"
else bad "--purge without confirmation keeps the data"; fi

# 13. --purge --yes deletes the data
if run_installer --uninstall --purge --yes && [ ! -e "$WORK/config" ]; then
	ok "--purge --yes deletes the data"
else bad "--purge --yes deletes the data"; fi

printf '\n%d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
