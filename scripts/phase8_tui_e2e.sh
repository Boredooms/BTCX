#!/usr/bin/env bash
# phase8_tui_e2e.sh — Phase 8 TUI end-to-end acceptance (offline).
#
# Proves the Phase 8 investigator terminal works end to end on LIVE local data
# with NO network, and that it tells the truth about network/acquisition state
# (AGENTS §16 no fake live data, §17 offline truthfulness). It:
#
#   1. Seeds a fixture case with locally-owned canonical data (a short wallet
#      chain), exactly the post-acquisition state the TUI renders.
#   2. Drives the TUI's non-TTY status path (`bctx tui` piped) under --offline
#      and asserts the honest status surface: NETWORK DISCONNECTED, MODELS
#      LOADED (resolved from the local registry, not "PENDING"), the real corpus
#      counts, and a READY graph — no fabricated "LIVE" or connected state.
#   3. Drives the geo/map CLIs the TUI's Geo Map screen depends on, fully
#      offline: honest NOT-INSTALLED degrade, install round-trip, provenance,
#      and sha256 verify — against a throwaway HOME so the real ~/.bctx is never
#      touched.
#   4. Asserts the only two network-touching actions (sync, start monitor) are
#      BLOCKED with an honest message under --offline.
#
# Model: scripts/phase7_offline_e2e.sh + scripts/phase8_geo_map_smoke.sh harness
# (set -u, PATH export, mktemp workspace + throwaway HOME, PASS/FAIL counter,
# non-zero exit on any failure). Does not modify or depend on those scripts.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
cd /home/devara/btcx

BIN=./bin/bctx
MAPFIX="tui/mapdata/testdata/world-110m.asset"
WORK=$(mktemp -d /tmp/bctx-p8-tui-e2e.XXXXXX)
CFG="$WORK/config.toml"
SEED="$WORK/seed.csv"
ADDR=A

# Throwaway HOME so geo/map install writes under $WORK, never the real ~/.bctx.
export HOME="$WORK/home"
mkdir -p "$HOME"

PASS=0
FAIL=0
ok()  { echo "PASS: $1"; PASS=$((PASS+1)); }
bad() { echo "FAIL: $1"; echo "   $2"; FAIL=$((FAIL+1)); }

echo "== workspace: $WORK  (HOME=$HOME) =="

# Build the binary if it is missing (building needs Go; running does not).
if [ ! -x "$BIN" ]; then
  echo "== build bctx =="
  export CGO_ENABLED=1
  go build -o "$BIN" ./apps/bctx || { bad "build" "go build failed"; echo "== SUMMARY: PASS=$PASS FAIL=$FAIL =="; rm -rf "$WORK"; exit 1; }
fi

# ---------------------------------------------------------------------------
# 1. Seed a fixture case with local canonical data. Online config so the ONLY
#    thing forcing offline behavior is the --offline flag — exactly what we test.
# ---------------------------------------------------------------------------
cat > "$CFG" <<EOF
[app]
data_dir = "$WORK/data"
log_level = "error"

[network]
acquisition_enabled = true
mode = "online"

[acquisition]
provider = "fake"
EOF

cat > "$SEED" <<EOF
txid,timestamp,fee_btc,input_address,input_amount,output_address,output_amount
T1,2026-01-01T10:00:00Z,0.0001,A,2.00000000,B,1.90000000
T2,2026-01-02T11:00:00Z,0.0002,B,1.90000000,C,1.70000000
T3,2026-01-03T12:00:00Z,0.0001,C,1.70000000,D,1.60000000
EOF

echo
echo "== SETUP: init + case + import + graph (offline ingestion) =="
$BIN --config "$CFG" init >/dev/null 2>&1 || bad "init" "init failed"
$BIN --config "$CFG" case create p8 >/dev/null 2>&1 || bad "case create" "case create failed"
OUT=$($BIN --config "$CFG" dataset import "$SEED" --format csv 2>&1); RC=$?
[ $RC -eq 0 ] && ok "canonical data landed locally (import)" || bad "import rc=$RC" "$OUT"
OUT=$($BIN --config "$CFG" graph build 2>&1); RC=$?
[ $RC -eq 0 ] && ok "graph built from local data" || bad "graph build rc=$RC" "$OUT"

# ---------------------------------------------------------------------------
# 2. TUI status path (non-TTY) under --offline. Assert honest status + live
#    counts; assert NO fake connected/LIVE state.
# ---------------------------------------------------------------------------
echo
echo "== TUI status path (bctx tui, piped, --offline) =="
TUIOUT="$WORK/tui.txt"
$BIN --config "$CFG" --offline tui > "$TUIOUT" 2>&1 || bad "tui run" "bctx tui exited non-zero"

grep -q "Network:      DISCONNECTED" "$TUIOUT" && ok "TUI reports NETWORK DISCONNECTED honestly (offline)" \
  || bad "tui network" "$(cat "$TUIOUT")"
grep -q "Case:         p8" "$TUIOUT" && ok "TUI shows the active case" \
  || bad "tui case" "$(cat "$TUIOUT")"
grep -q "Transactions: 3" "$TUIOUT" && ok "TUI shows live transaction count (3)" \
  || bad "tui tx count" "$(cat "$TUIOUT")"
grep -q "Models:       LOADED" "$TUIOUT" && ok "TUI resolves MODELS from local registry (LOADED, not PENDING)" \
  || bad "tui models" "$(cat "$TUIOUT")"
grep -q "Graph:        READY" "$TUIOUT" && ok "TUI shows graph READY" \
  || bad "tui graph" "$(cat "$TUIOUT")"
# No fake live/connected language while offline. Match CONNECTED only as a whole
# word (so DISCONNECTED does not false-positive) and LIVE as a whole word (so
# "Platform" and similar do not match).
if grep -Eqw "CONNECTED|LIVE" "$TUIOUT"; then
  bad "tui no-fake-live" "offline TUI output must not claim CONNECTED/LIVE: $(cat "$TUIOUT")"
else
  ok "TUI claims no fake CONNECTED/LIVE state while offline"
fi

# The bare `bctx` entrypoint prints the same honest summary.
BAREOUT=$($BIN --config "$CFG" --offline 2>&1)
grep -q "Network:      DISCONNECTED" <<<"$BAREOUT" && ok "bare bctx entrypoint matches the honest TUI summary" \
  || bad "bare bctx" "$BAREOUT"

# ---------------------------------------------------------------------------
# 3. Geo / Map CLIs (the Geo Map screen's local-asset backend), fully offline.
# ---------------------------------------------------------------------------
echo
echo "== GEO / MAP offline asset lifecycle =="
OUT=$($BIN --config "$CFG" geo status 2>&1)
grep -qF "GEOIP DATABASE NOT INSTALLED" <<<"$OUT" && ok "geo status degrades honestly (not installed)" \
  || bad "geo status" "$OUT"
OUT=$($BIN --config "$CFG" map status 2>&1)
grep -qF "WORLD GEOMETRY ASSET NOT INSTALLED" <<<"$OUT" && ok "map status degrades honestly (not installed)" \
  || bad "map status" "$OUT"

if [ -f "$MAPFIX" ]; then
  OUT=$($BIN --config "$CFG" map install "$MAPFIX" 2>&1)
  grep -qF "Installed world-110m" <<<"$OUT" && ok "map install records the fixture asset" \
    || bad "map install" "$OUT"
  OUT=$($BIN --config "$CFG" map status 2>&1)
  grep -qF "public domain" <<<"$OUT" && ok "map status shows provenance after install" \
    || bad "map status post-install" "$OUT"
  OUT=$($BIN --config "$CFG" map verify 2>&1)
  grep -qF "VERIFY OK" <<<"$OUT" && ok "map verify of installed asset is OK (sha256)" \
    || bad "map verify" "$OUT"
else
  bad "map fixture" "fixture asset $MAPFIX not found"
fi

# ---------------------------------------------------------------------------
# 4. The two network-touching actions are blocked offline with an honest msg.
# ---------------------------------------------------------------------------
echo
echo "== OFFLINE GATE: sync + monitor blocked honestly =="
OUT=$($BIN --config "$CFG" --offline sync wallet "$ADDR" 2>&1); RC=$?
if [ $RC -ne 0 ] && grep -qi "acquisition unavailable offline" <<<"$OUT"; then
  ok "sync blocked offline with an honest message"
else
  bad "sync gate" "rc=$RC out=$OUT"
fi
OUT=$($BIN --config "$CFG" --offline monitor wallet "$ADDR" 2>&1); RC=$?
if [ $RC -ne 0 ] && grep -qi "acquisition unavailable offline" <<<"$OUT"; then
  ok "start-monitor blocked offline with an honest message"
else
  bad "monitor gate" "rc=$RC out=$OUT"
fi

echo
echo "== SUMMARY: PASS=$PASS FAIL=$FAIL  (workspace $WORK) =="
rm -rf "$WORK"
[ $FAIL -eq 0 ]
