#!/usr/bin/env bash
# Phase 8.5 acceptance: acquire a REAL subject ONLINE, then do ALL analysis +
# reporting OFFLINE (the "get details, then go offline" model). Opt-in via
# BCTX_LIVE=1; falls back to the fake provider (still proves the offline half)
# when BCTX_LIVE is unset.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx
go build -o bin/bctx ./apps/bctx || exit 1
BIN=./bin/bctx

WORK=$(mktemp -d /tmp/bctx-acqrep.XXXXXX)
PROVIDER="fake"
ADDR="A"
if [ "${BCTX_LIVE:-0}" = "1" ]; then
  PROVIDER="esplora"
  # Satoshi's genesis address: real, immutable, modestly sized history.
  ADDR="1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
fi
cat > "$WORK/config.toml" <<EOF
[app]
data_dir = "$WORK/data"
log_level = "error"
[network]
acquisition_enabled = true
mode = "online"
[acquisition]
provider = "$PROVIDER"
page_size = 25
EOF
$BIN --config "$WORK/config.toml" init >/dev/null 2>&1
$BIN --config "$WORK/config.toml" case create acqrep >/dev/null 2>&1

PASS=0; FAIL=0
ok(){ echo "PASS: $1"; PASS=$((PASS+1)); }
bad(){ echo "FAIL: $1"; echo "  $2"; FAIL=$((FAIL+1)); }

echo "== 1: ONLINE acquisition of $ADDR via $PROVIDER =="
OUT=$($BIN --config "$WORK/config.toml" sync wallet "$ADDR" 2>&1)
echo "$OUT" | grep -qiE 'STATUS: *(completed|partial)' && ok "acquired online" || bad "acquire failed" "$OUT"

echo
echo "== 2: data landed locally (counts) =="
OUT=$($BIN --config "$WORK/config.toml" --json status 2>&1)
TX=$(echo "$OUT" | grep -o '\"transactions\": [0-9]*' | grep -o '[0-9]*')
[ "${TX:-0}" -gt 0 ] && ok "persisted $TX transactions" || bad "no local data" "$OUT"

echo
echo "== 3: OFFLINE analysis (no network) — risk band present =="
OUT=$($BIN --config "$WORK/config.toml" --offline --json analyze wallet "$ADDR" 2>&1)
RC=$?
BAND=$(echo "$OUT" | grep -o '\"band\": \"[A-Z]*\"' | head -1 | grep -o '[A-Z]*')
SCORE=$(echo "$OUT" | grep -o '\"score\": [0-9]*' | head -1 | grep -o '[0-9]*')
[ $RC -eq 0 ] && [ -n "$BAND" ] && ok "offline analyze ok (risk $SCORE / $BAND)" || bad "offline analyze failed" "$OUT"

echo
echo "== 4: OFFLINE report generate (json + markdown + pdf) =="
for fmt in json md pdf; do
  OUT=$($BIN --config "$WORK/config.toml" --offline report generate "$ADDR" --format "$fmt" 2>&1); RC=$?
  [ $RC -eq 0 ] && ok "offline report $fmt" || bad "report $fmt failed" "$OUT"
done

echo
echo "== 5: OFFLINE export bundle + verify =="
OUT=$($BIN --config "$WORK/config.toml" --offline report export "$ADDR" 2>&1); RC=$?
BUNDLE=$(echo "$OUT" | grep -oE '/[^ ]*report' | head -1)
[ $RC -eq 0 ] && ok "offline export bundle" || bad "export failed" "$OUT"
if [ -n "$BUNDLE" ]; then
  OUT=$($BIN --config "$WORK/config.toml" --offline report verify "$BUNDLE" 2>&1); RC=$?
  echo "$OUT" | grep -qiE 'ok|verified|pass' && ok "offline verify" || bad "verify failed" "$OUT"
fi

echo
echo "== SUMMARY: PASS=$PASS FAIL=$FAIL (provider=$PROVIDER) =="
rm -rf "$WORK"
[ $FAIL -eq 0 ]
