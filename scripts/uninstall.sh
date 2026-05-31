#!/usr/bin/env bash
#
# drawpad uninstaller
# ───────────────────
# Removes every drawpad binary the installer might have placed, and reverses
# any PATH edits made by scripts/install.sh.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/merijjeyn/drawpad/main/scripts/uninstall.sh | bash
#
# Will check, in order:
#   $DRAWPAD_INSTALL_DIR (if set)
#   $HOME/.local/bin
#   /usr/local/bin
#   /opt/homebrew/bin
#   $(go env GOPATH)/bin   (only if `go` is on PATH)
#
# Removes shell-rc lines that match the marker the installer wrote.
# Never edits a file unless the marker is present, so manual edits are safe.
#
# Optional environment variables:
#   DRAWPAD_INSTALL_DIR        Extra dir to check before the standard list.
#   DRAWPAD_KEEP_PATH_EDITS=1  Don't touch any shell rc files.

set -euo pipefail

BIN_NAME="drawpad"
KEEP_PATH_EDITS="${DRAWPAD_KEEP_PATH_EDITS:-0}"

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
skip() { printf "%s·%s %s\n" "$DIM" "$RESET" "$*"; }
warn() { printf "%s!%s %s\n" "$YELLOW" "$RESET" "$*" >&2; }

# ── locate every install ─────────────────────────────────────────────────
dirs=()
[ -n "${DRAWPAD_INSTALL_DIR:-}" ] && dirs+=("$DRAWPAD_INSTALL_DIR")
dirs+=("$HOME/.local/bin" "/usr/local/bin" "/opt/homebrew/bin")
if command -v go >/dev/null 2>&1; then
  gobin="$(go env GOPATH 2>/dev/null)/bin"
  [ -n "$gobin" ] && dirs+=("$gobin")
fi

removed_any=0
for d in "${dirs[@]}"; do
  bin="$d/$BIN_NAME"
  if [ -f "$bin" ] || [ -L "$bin" ]; then
    info "Removing $bin"
    if rm -f "$bin" 2>/dev/null; then
      ok "removed $bin"
      removed_any=1
    else
      # /usr/local/bin needs sudo on most setups.
      warn "permission denied; retrying with sudo"
      sudo rm -f "$bin" && ok "removed $bin (with sudo)" && removed_any=1
    fi
  else
    skip "not in $d"
  fi
done

[ "$removed_any" = 1 ] || warn "no drawpad binary found in the usual locations"

# ── reverse PATH edits the installer made ────────────────────────────────
if [ "$KEEP_PATH_EDITS" = 1 ]; then
  skip "leaving shell rc files untouched (DRAWPAD_KEEP_PATH_EDITS=1)"
else
  rcs=("$HOME/.zshrc" "$HOME/.bashrc" "$HOME/.bash_profile" "$HOME/.config/fish/config.fish")
  marker_re='^# Added by drawpad installer on '
  rcs_edited=0
  for rc in "${rcs[@]}"; do
    [ -f "$rc" ] || continue
    if ! grep -qE "$marker_re" "$rc"; then
      continue
    fi
    info "Removing installer-added PATH line from $rc"
    # Delete each marker comment and the single line immediately after.
    # Use awk for portability between BSD (macOS) and GNU sed.
    tmp="$(mktemp)"
    awk -v re="$marker_re" '
      $0 ~ re { skip = 1; next }
      skip    { skip = 0; next }
      { print }
    ' "$rc" >"$tmp"
    mv "$tmp" "$rc"
    ok "cleaned $rc"
    rcs_edited=1
  done
  [ "$rcs_edited" = 1 ] || skip "no installer-marked PATH edits found in shell rc files"
fi

say
if [ "$removed_any" = 1 ]; then
  say "${BOLD}Uninstalled.${RESET} Open a new terminal so PATH changes take effect."
else
  say "${BOLD}Nothing to do.${RESET}"
fi

# Sanity check: if `drawpad` is still resolvable, the user probably installed
# it through a path we don't manage (Homebrew tap, manual /usr/bin, etc.).
if command -v drawpad >/dev/null 2>&1; then
  say
  warn "'drawpad' is still on PATH at $(command -v drawpad)."
  warn "It was installed via a method this script doesn't manage (e.g. Homebrew). Remove it manually."
fi
