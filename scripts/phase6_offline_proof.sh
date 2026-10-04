#!/usr/bin/env bash
# Phase 6 offline hard-gate proof. Runs the real monitor command with --offline
# and under a network-isolated namespace, asserting it refuses to acquire while
# local analysis-only commands still work.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
cd /home/devara/btcx

BIN=./bin/bctx
WORK=$(mktemp -d /tmp/bctx-offproof.XXXXXX)
CFG="$WORK/config.toml"

echo "== workspace: $WORK =="

# Minimal config rooted inside the temp dir (isolated from ~/.bctx). Online by
# default, so the ONLY thing that can block acquisition is the --offline/--airgap
# flag -- which is exactly what we are proving.
cat > "$CFG" <<EOF
[app]
data_dir = "$WORK/data"
log_level = "error"

[network]
acquisition_enabled = true
mode = "online"

[acquisition]
provider = "esplora"
EOF

$BIN --config "$CFG" init >/dev/null 2>&1
$BIN --config "$CFG" case create proof >/dev/null 2>&1

PASS=0; FAIL=0
check() { if echo "$3" | grep -qi "$2"; then echo "PASS: $1"; PASS=$((PASS+1));
  else echo "FAIL: $1"; echo "   want substr: $2"; echo "   got: $3"; FAIL=$((FAIL+1)); fi; }

echo
echo "== GATE 1: monitor wallet --offline must refuse =="
OUT=$($BIN --config "$CFG" --offline monitor wallet A --max-polls 1 2>&1); echo "exit=$?"
check "offline refuses with offline error" "unavailable offline" "$OUT"

echo
echo "== GATE 2: monitor wallet --airgap must refuse =="
OUT=$($BIN --config "$CFG" --airgap monitor wallet A --max-polls 1 2>&1)
check "airgap refuses with offline error" "unavailable offline" "$OUT"

echo
echo "== GATE 3: monitor wallet under unshare -rn (no network ns) + --offline =="
if command -v unshare >/dev/null 2>&1; then
  OUT=$(unshare -rn "$BIN" --config "$CFG" --offline monitor wallet A --max-polls 1 2>&1)
  check "unshare+offline refuses cleanly (no panic/network)" "unavailable offline" "$OUT"
else
  echo "SKIP: unshare unavailable"
fi

echo
echo "== GATE 4: provider list works offline (no network) =="
OUT=$($BIN --config "$CFG" --offline provider list 2>&1); RC=$?; echo "exit=$RC"
if [ $RC -eq 0 ]; then echo "PASS: provider list runs offline"; PASS=$((PASS+1));
else echo "FAIL: provider list exit=$RC out=$OUT"; FAIL=$((FAIL+1)); fi

echo
echo "== GATE 5: monitor list works offline (local state only) =="
OUT=$($BIN --config "$CFG" --offline monitor list 2>&1); RC=$?; echo "exit=$RC"
if [ $RC -eq 0 ]; then echo "PASS: monitor list runs offline"; PASS=$((PASS+1));
else echo "FAIL: monitor list exit=$RC out=$OUT"; FAIL=$((FAIL+1)); fi

echo
echo "== SUMMARY: PASS=$PASS FAIL=$FAIL =="
rm -rf "$WORK"
[ $FAIL -eq 0 ]
