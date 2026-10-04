#!/usr/bin/env bash
# Phase 6 LIVE end-to-end proof: real Esplora provider -> canonical persist ->
# incremental graph -> analysis, all through the real CLI binary. Opt-in via
# BCTX_LIVE=1 (opens real network connections to blockstream.info).
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
cd /home/devara/btcx

if [ "${BCTX_LIVE:-0}" != "1" ]; then
  echo "SKIP: set BCTX_LIVE=1 to run the live e2e proof"; exit 0
fi

BIN=./bin/bctx
WORK=$(mktemp -d /tmp/bctx-live-e2e.XXXXXX)
CFG="$WORK/config.toml"
# A real, modestly-sized confirmed address (Satoshi's genesis address).
ADDR="1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"

cat > "$CFG" <<EOF
[app]
data_dir = "$WORK/data"
log_level = "error"

[network]
acquisition_enabled = true
mode = "online"

[acquisition]
provider = "esplora"
page_size = 25
EOF

echo "== workspace: $WORK (provider=esplora, addr=$ADDR) =="
$BIN --config "$CFG" init >/dev/null 2>&1
$BIN --config "$CFG" case create live >/dev/null 2>&1

PASS=0; FAIL=0
ok()   { echo "PASS: $1"; PASS=$((PASS+1)); }
bad()  { echo "FAIL: $1"; echo "   $2"; FAIL=$((FAIL+1)); }

echo
echo "== 1: provider list shows the real esplora provider =="
OUT=$($BIN --config "$CFG" provider list 2>&1)
echo "$OUT" | grep -qi esplora && ok "esplora listed" || bad "esplora not listed" "$OUT"

echo
echo "== 2: live monitor one poll against the real chain (JSON) =="
OUT=$($BIN --config "$CFG" --json monitor wallet "$ADDR" --max-polls 1 2>&1)
echo "$OUT"
echo "$OUT" | grep -q '"provider": "esplora"' && ok "monitored via esplora" || bad "provider not esplora" "$OUT"
NEWTX=$(echo "$OUT" | grep -o '"new_transactions": [0-9]*' | grep -o '[0-9]*')
[ "${NEWTX:-0}" -gt 0 ] && ok "acquired $NEWTX real transactions" || bad "no transactions acquired" "$OUT"

echo
echo "== 3: canonical data landed locally (dataset list runs offline) =="
OUT=$($BIN --config "$CFG" --offline dataset list 2>&1); RC=$?
echo "$OUT"
[ $RC -eq 0 ] && ok "dataset list runs offline (local data owned)" || bad "dataset list failed rc=$RC" "$OUT"

echo
echo "== 4: OFFLINE analysis works on acquired data (no network) =="
OUT=$($BIN --config "$CFG" --offline analyze wallet "$ADDR" 2>&1)
RC=$?
echo "$OUT" | head -20
[ $RC -eq 0 ] && ok "offline analyze succeeded on live-acquired data" || bad "offline analyze failed rc=$RC" "$OUT"

echo
echo "== 5: second monitor poll is idempotent (0 new tx) =="
OUT=$($BIN --config "$CFG" --json monitor wallet "$ADDR" --max-polls 1 2>&1)
NEW2=$(echo "$OUT" | grep -o '"new_transactions": [0-9]*' | grep -o '[0-9]*')
[ "${NEW2:-99}" -eq 0 ] && ok "re-poll acquired 0 new (idempotent)" || bad "re-poll not idempotent: $NEW2" "$OUT"

echo
echo "== SUMMARY: PASS=$PASS FAIL=$FAIL  (workspace $WORK) =="
rm -rf "$WORK"
[ $FAIL -eq 0 ]
