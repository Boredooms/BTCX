#!/usr/bin/env bash
# Ensure ~/.local/bin is on PATH (persisted in ~/.bashrc) and verify that both
# `btcx` and `bctx` are runnable from a NON-repo directory (cwd-independence).
set -u

LINE='export PATH="$HOME/.local/bin:$PATH"'
if ! grep -qF '.local/bin' "$HOME/.bashrc" 2>/dev/null; then
  printf '\n# BCTX global launch\n%s\n' "$LINE" >> "$HOME/.bashrc"
  echo "added ~/.local/bin to PATH in ~/.bashrc"
else
  echo "~/.local/bin already referenced in ~/.bashrc"
fi

export PATH="$HOME/.local/bin:$PATH"
cd /tmp || exit 2

echo "--- which ---"
which btcx || echo "btcx NOT found"
which bctx || echo "bctx NOT found"

echo "--- running 'btcx status' from $(pwd) ---"
btcx status 2>&1 | head -15

echo "--- running 'btcx version' from $(pwd) ---"
btcx version 2>&1 | head -3
