#!/usr/bin/env bash
#
# drawpad installer
# ─────────────────
# Downloads the latest (or pinned) drawpad release from GitHub, verifies its
# SHA-256 checksum, drops the binary into $DRAWPAD_INSTALL_DIR (default
# ~/.local/bin), and ensures that directory is on your PATH.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/merijjeyn/drawpad/main/scripts/install.sh | bash
#
# Optional environment variables:
#   DRAWPAD_VERSION       Pin a version, e.g. v0.1.0. Default: latest.
#   DRAWPAD_INSTALL_DIR   Install dir. Default: $HOME/.local/bin.
#   DRAWPAD_NO_MODIFY_PATH=1   Don't touch shell rc files; just print
#                              the line you should add yourself.

set -euo pipefail

REPO="merijjeyn/drawpad"
BIN_NAME="drawpad"

VERSION="${DRAWPAD_VERSION:-latest}"
INSTALL_DIR="${DRAWPAD_INSTALL_DIR:-$HOME/.local/bin}"
NO_MODIFY_PATH="${DRAWPAD_NO_MODIFY_PATH:-0}"

# ── pretty output ────────────────────────────────────────────────────────
if [ -t 1 ]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'
  GREEN=$'\033[32m'; YELLOW=$'\033[33m'; BLUE=$'\033[34m'; RESET=$'\033[0m'
else
  BOLD=""; DIM=""; RED=""; GREEN=""; YELLOW=""; BLUE=""; RESET=""
fi

say()  { printf "%s\n" "$*"; }
info() { printf "%s==>%s %s\n" "$BLUE" "$RESET" "$*"; }
ok()   { printf "%s✓%s %s\n" "$GREEN" "$RESET" "$*"; }
warn() { printf "%s!%s %s\n" "$YELLOW" "$RESET" "$*" >&2; }
die()  { printf "%serror:%s %s\n" "$RED" "$RESET" "$*" >&2; exit 1; }

# ── preflight ────────────────────────────────────────────────────────────
for cmd in curl tar uname mkdir mv chmod; do
  command -v "$cmd" >/dev/null 2>&1 || die "missing required command: $cmd"
done

# sha256: prefer sha256sum (linux), fall back to shasum (macOS/BSD).
if command -v sha256sum >/dev/null 2>&1; then
  SHA_CMD="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  SHA_CMD="shasum -a 256"
else
  die "need either sha256sum or shasum on PATH for checksum verification"
fi

# ── detect OS / arch ─────────────────────────────────────────────────────
uname_s="$(uname -s)"
uname_m="$(uname -m)"
case "$uname_s" in
  Darwin) os="macOS" ;;
  Linux)  os="linux" ;;
  *) die "unsupported OS: $uname_s. Windows users: download the .zip from https://github.com/$REPO/releases manually." ;;
esac
case "$uname_m" in
  x86_64|amd64) arch="x86_64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) die "unsupported architecture: $uname_m" ;;
esac
info "Detected ${BOLD}${os}/${arch}${RESET}"

# ── resolve version ──────────────────────────────────────────────────────
if [ "$VERSION" = "latest" ]; then
  # Use the /releases/latest redirect to learn the tag name without hitting
  # the GitHub API (which would rate-limit unauthenticated users).
  resolved="$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
    "https://github.com/$REPO/releases/latest")" \
    || die "could not contact github.com to resolve latest version"
  VERSION="${resolved##*/}"
  [ -n "$VERSION" ] || die "could not parse latest tag from redirect URL"
fi
info "Installing drawpad ${BOLD}${VERSION}${RESET}"

# Tag (vX.Y.Z) → bare version used in the archive filename (X.Y.Z).
ver_bare="${VERSION#v}"
archive="${BIN_NAME}_${ver_bare}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$VERSION/$archive"
checksums_url="https://github.com/$REPO/releases/download/$VERSION/checksums.txt"

# ── download + verify ────────────────────────────────────────────────────
tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t drawpad-install)"
trap 'rm -rf "$tmpdir"' EXIT

info "Downloading $archive"
curl -fsSL "$url" -o "$tmpdir/$archive" \
  || die "download failed: $url (does that release asset exist?)"

info "Verifying checksum"
curl -fsSL "$checksums_url" -o "$tmpdir/checksums.txt" \
  || die "could not download checksums.txt from $checksums_url"

expected="$(awk -v f="$archive" '$2 == f { print $1 }' "$tmpdir/checksums.txt")"
[ -n "$expected" ] || die "no checksum line for $archive in checksums.txt"
actual="$( ($SHA_CMD "$tmpdir/$archive") | awk '{print $1}')"
if [ "$expected" != "$actual" ]; then
  die "checksum mismatch! expected $expected, got $actual"
fi
ok "checksum OK ($expected)"

# ── extract + install ────────────────────────────────────────────────────
tar -xzf "$tmpdir/$archive" -C "$tmpdir"
[ -f "$tmpdir/$BIN_NAME" ] || die "binary '$BIN_NAME' not found inside archive"

mkdir -p "$INSTALL_DIR"
mv "$tmpdir/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
chmod +x "$INSTALL_DIR/$BIN_NAME"
ok "installed $INSTALL_DIR/$BIN_NAME"

# ── PATH handling ────────────────────────────────────────────────────────
on_path=0
case ":$PATH:" in
  *":$INSTALL_DIR:"*) on_path=1 ;;
esac

if [ "$on_path" = 1 ]; then
  ok "$INSTALL_DIR is already on your PATH"
  say
  say "${BOLD}Done.${RESET} Try it:"
  say "    drawpad --version"
  say "    drawpad --prompt \"hello\""
  exit 0
fi

# Detect shell + rc file. Prefer $SHELL because users may run this from
# inside a different interpreter than their login shell.
shell_name="$(basename "${SHELL:-/bin/bash}")"
case "$shell_name" in
  zsh)  rc="$HOME/.zshrc" ;;
  bash)
    # On macOS, interactive bash reads .bash_profile; on Linux, .bashrc.
    if [ "$uname_s" = "Darwin" ] && [ -f "$HOME/.bash_profile" ]; then
      rc="$HOME/.bash_profile"
    else
      rc="$HOME/.bashrc"
    fi
    ;;
  fish) rc="$HOME/.config/fish/config.fish" ;;
  *)    rc="" ;;
esac

# The line we'll add. Use double-quoting around $PATH so the literal token
# survives. For fish the syntax differs.
if [ "$shell_name" = "fish" ]; then
  path_line="fish_add_path \"$INSTALL_DIR\""
else
  path_line="export PATH=\"$INSTALL_DIR:\$PATH\""
fi

warn "$INSTALL_DIR is not on your PATH yet."

if [ "$NO_MODIFY_PATH" = "1" ] || [ -z "$rc" ]; then
  say
  say "Add this line to your shell rc to fix it:"
  say "    ${BOLD}$path_line${RESET}"
  [ -n "$rc" ] && say "  (typically in ${DIM}$rc${RESET})"
  say
  say "Then reload your shell, or open a new terminal."
  exit 0
fi

# Idempotency: skip the append if the exact line is already there.
if [ -f "$rc" ] && grep -Fqx "$path_line" "$rc"; then
  ok "PATH line already present in $rc — just reload your shell"
  exit 0
fi

info "Appending PATH update to $rc"
{
  printf '\n# Added by drawpad installer on %s\n' "$(date '+%Y-%m-%d')"
  printf '%s\n' "$path_line"
} >>"$rc"
ok "updated $rc"

say
say "${BOLD}Done.${RESET} Open a new terminal (or run ${BOLD}source $rc${RESET}), then:"
say "    drawpad --version"
say "    drawpad --prompt \"hello\""
say
say "${DIM}Tip: set DRAWPAD_NO_MODIFY_PATH=1 next time to skip the rc edit.${RESET}"
