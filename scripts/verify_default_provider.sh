#!/usr/bin/env bash
# Verify a fresh `bctx init` ships the real esplora provider by default and
# that airgap/offline still refuse the network before provider construction.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2
BIN=./bin/bctx

TMPH="$(mktemp -d)"
trap 'rm -rf "$TMPH"' EXIT
export HOME="$TMPH"
export XDG_CONFIG_HOME="$TMPH/.config"

PASS=0; FAIL=0
chk() { if eval "$2"; then echo "  PASS: $1"; PASS=$((PASS+1)); else echo "  FAIL: $1"; FAIL=$((FAIL+1)); fi; }

"$BIN" init >/dev/null 2>&1
CFG="$XDG_CONFIG_HOME/bctx/config.toml"

echo "=== fresh config [acquisition] ==="
grep -A1 '\[acquisition\]' "$CFG" || true

chk "fresh config selects esplora provider" "grep -q 'provider = \"esplora\"' '$CFG'"
chk "fresh config pins blockstream endpoint" "grep -q 'blockstream.info/api' '$CFG'"

OUT_AIRGAP="$("$BIN" --airgap sync tx fb070dcdd26715c8dfd26ad4fbd4ff199764e86ffc179a1e3b53ceee3b64ae14 2>&1)"
chk "airgap refuses acquisition" "echo \"$OUT_AIRGAP\" | grep -qi 'unavailable offline'"

OUT_OFFLINE="$("$BIN" --offline sync tx fb070dcdd26715c8dfd26ad4fbd4ff199764e86ffc179a1e3b53ceee3b64ae14 2>&1)"
chk "offline refuses acquisition" "echo \"$OUT_OFFLINE\" | grep -qi 'unavailable offline'"

echo "== SUMMARY: PASS=$PASS FAIL=$FAIL =="
[ "$FAIL" -eq 0 ]
