#!/usr/bin/env bash
# Install BCTX so `btcx` (and `bctx`) launch the TUI from ANY directory.
#
# BCTX is cwd-independent: config lives at ~/.config/bctx/config.toml and all
# data under ~/.bctx (resolved from the user's home, never the working dir), so
# once the binary is on PATH it opens the same workstation from anywhere.
#
# Running the binary with no subcommand launches the interactive TUI
# (cli/commands/root.go RunE -> runHome), so `btcx` with no args IS the app.
#
# Usage:
#   bash scripts/install_btcx.sh            # build + install to the best PATH dir
#   PREFIX=/usr/local/bin bash scripts/install_btcx.sh   # force a target dir
#
# No network, no sudo unless the chosen PREFIX needs it.
set -euo pipefail

export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin:${PATH:-}
export CGO_ENABLED=1

REPO="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO"

echo "==> building bctx (CGO on)"
go build -o bin/bctx ./apps/bctx
BIN="$REPO/bin/bctx"

# Pick an install directory already on PATH, preferring a user-writable one so
# no sudo is needed. Order: explicit PREFIX, ~/.local/bin, /usr/local/bin.
choose_dir() {
  if [ -n "${PREFIX:-}" ]; then echo "$PREFIX"; return; fi
  if [ -d "$HOME/.local/bin" ] || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
    echo "$HOME/.local/bin"; return
  fi
  echo "/usr/local/bin"
}
DEST="$(choose_dir)"
mkdir -p "$DEST" 2>/dev/null || true

install_one() {
  local name="$1" target="$DEST/$1"
  if cp "$BIN" "$target" 2>/dev/null; then
    chmod +x "$target"
    echo "==> installed $target"
  elif command -v sudo >/dev/null 2>&1; then
    sudo cp "$BIN" "$target" && sudo chmod +x "$target"
    echo "==> installed $target (via sudo)"
  else
    echo "!! could not write $target (no permission, no sudo)"; return 1
  fi
}

# Install BOTH spellings: bctx (canonical) and btcx (the common typo / brand).
install_one bctx
install_one btcx

# PATH hint if the chosen dir is not on PATH yet.
case ":$PATH:" in
  *":$DEST:"*) : ;;
  *)
    echo
    echo "NOTE: $DEST is not on your PATH. Add this to ~/.bashrc (or ~/.zshrc):"
    echo "    export PATH=\"$DEST:\$PATH\""
    ;;
esac

echo
echo "Done. From ANY directory you can now run:"
echo "    btcx            # launch the interactive TUI"
echo "    btcx status     # or any subcommand"
echo "    bctx --help"
