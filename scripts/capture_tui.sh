#!/usr/bin/env bash
# Capture ONE frame of the real bctx TUI under a PTY at a forced size, so we see
# exactly what the user sees (not the test harness). Uses `script` for a PTY and
# tmux is not required. Sends 'q' after a short delay to quit cleanly.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
cd /home/devara/btcx

COLS=${1:-237}
ROWS=${2:-63}

# tmux gives a reliable fixed-size PTY and a capture-pane command.
if command -v tmux >/dev/null 2>&1; then
  tmux kill-session -t bctxcap 2>/dev/null || true
  tmux new-session -d -s bctxcap -x "$COLS" -y "$ROWS"
  tmux send-keys -t bctxcap "cd /home/devara/btcx && ./bin/bctx" C-m
  sleep 4
  tmux capture-pane -t bctxcap -p > /tmp/tui_frame.txt
  tmux send-keys -t bctxcap "q" 2>/dev/null || true
  sleep 1
  tmux kill-session -t bctxcap 2>/dev/null || true
  echo "=== captured ${COLS}x${ROWS} ==="
  cat /tmp/tui_frame.txt
else
  echo "tmux not available"
fi
