#!/usr/bin/env bash
# phase8_airgap_tui.sh — Phase 8 TUI air-gap proof.
#
# Proves the Phase 8 terminal and its local-asset backends (geo/map) run with
# NO network at all, under a dropped network namespace (`unshare -rn`), and that
# the only two network-touching actions (sync, start monitor) are BLOCKED with
# an honest message — never a silent failure, never a fabricated result
# (AGENTS §16/§17). The TUI itself opens no socket (enforced at build time by
# tests/offline TestTUIPackagesHaveNoHTTPTransport); this script is the runtime
# complement: even where a socket is physically impossible, the TUI status path
# and the geo/map CLIs still work on local data and the gate still blocks.
#
# If `unshare -rn` is unavailable, it falls back to --airgap + BCTX_OFFLINE and
# prints a clear note that the flag fallback was used (AGENTS: never hide the
# distinction).
#
# Model: scripts/phase7_airgap_bundle.sh harness (set -u, PATH export, mktemp
# workspace + throwaway HOME, PASS/FAIL counter, unshare-or-fallback, non-zero
# exit on any failure). Does not modify or depend on it.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
cd /home/devara/btcx

BIN=./bin/bctx
MAPFIX="tui/mapdata/testdata/world-110m.asset"
WORK=$(mktemp -d /tmp/bctx-p8-airgap-tui.XXXXXX)
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

if [ ! -x "$BIN" ]; then
  echo "== build bctx =="
  export CGO_ENABLED=1
  go build -o "$BIN" ./apps/bctx || { bad "build" "go build failed"; echo "== SUMMARY: PASS=$PASS FAIL=$FAIL =="; rm -rf "$WORK"; exit 1; }
fi

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
EOF

echo
echo "== SETUP (connected-era ownership): init + case + import + graph =="
$BIN --config "$CFG" init >/dev/null 2>&1 || bad "init" "init failed"
$BIN --config "$CFG" case create airgap >/dev/null 2>&1 || bad "case create" "case create failed"
$BIN --config "$CFG" dataset import "$SEED" --format csv >/dev/null 2>&1 || bad "import" "import failed"
$BIN --config "$CFG" graph build >/dev/null 2>&1 || bad "graph build" "graph build failed"
# Install the map asset while still outside the namespace (install is pure local
# file copy; this lets the air-gapped map status/verify exercise a real asset).
if [ -f "$MAPFIX" ]; then
  $BIN --config "$CFG" map install "$MAPFIX" >/dev/null 2>&1 || bad "map install" "install failed"
  ok "local case + map asset prepared"
else
  bad "map fixture" "fixture asset $MAPFIX not found"
fi

# ---------------------------------------------------------------------------
# Air-gap runner. runair <wrapper...> runs the TUI status path + geo/map CLIs
# and asserts sync/monitor are blocked. The wrapper is the isolation mechanism
# (unshare -rn, or env BCTX_OFFLINE=1 for the fallback); $BFLAG is the bctx flag.
# ---------------------------------------------------------------------------
runair() {
  local rc=0

  echo "-- [airgap] TUI status path works on local data --"
  "$@" $BIN --config "$CFG" "$BFLAG" tui > "$WORK/tui.txt" 2>&1 || rc=1
  if grep -q "Network:      " "$WORK/tui.txt" && grep -q "Models:       LOADED" "$WORK/tui.txt"; then
    ok "[airgap] TUI status renders on local data (no socket)"
  else
    bad "[airgap] tui status" "$(cat "$WORK/tui.txt")"; rc=1
  fi
  # Under --airgap the network reads AIRGAPPED; under --offline it is DISCONNECTED.
  if grep -Eq "Network:      (DISCONNECTED|AIRGAPPED)" "$WORK/tui.txt"; then
    ok "[airgap] TUI reports an honest offline network state"
  else
    bad "[airgap] tui network honesty" "$(cat "$WORK/tui.txt")"; rc=1
  fi
  if grep -Eqw "CONNECTED|LIVE" "$WORK/tui.txt"; then
    bad "[airgap] tui no-fake-live" "air-gapped TUI must not claim CONNECTED/LIVE"; rc=1
  else
    ok "[airgap] TUI claims no fake CONNECTED/LIVE state"
  fi

  echo "-- [airgap] geo/map local-asset CLIs work with no network --"
  "$@" $BIN --config "$CFG" "$BFLAG" geo status > "$WORK/geo.txt" 2>&1 || rc=1
  grep -qF "GEOIP DATABASE NOT INSTALLED" "$WORK/geo.txt" && ok "[airgap] geo status honest (no DB) with no dial" \
    || { bad "[airgap] geo status" "$(cat "$WORK/geo.txt")"; rc=1; }
  "$@" $BIN --config "$CFG" "$BFLAG" map verify > "$WORK/map.txt" 2>&1 || rc=1
  grep -qF "VERIFY OK" "$WORK/map.txt" && ok "[airgap] map verify OK from local asset with no dial" \
    || { bad "[airgap] map verify" "$(cat "$WORK/map.txt")"; rc=1; }

  echo "-- [airgap] sync + start-monitor BLOCKED with an honest message --"
  "$@" $BIN --config "$CFG" "$BFLAG" sync wallet "$ADDR" > "$WORK/sync.txt" 2>&1
  if [ $? -ne 0 ] && grep -qi "acquisition unavailable offline" "$WORK/sync.txt"; then
    ok "[airgap] sync blocked honestly (no dial attempted)"
  else
    bad "[airgap] sync gate" "$(cat "$WORK/sync.txt")"; rc=1
  fi
  "$@" $BIN --config "$CFG" "$BFLAG" monitor wallet "$ADDR" > "$WORK/mon.txt" 2>&1
  if [ $? -ne 0 ] && grep -qi "acquisition unavailable offline" "$WORK/mon.txt"; then
    ok "[airgap] start-monitor blocked honestly (no dial attempted)"
  else
    bad "[airgap] monitor gate" "$(cat "$WORK/mon.txt")"; rc=1
  fi

  return $rc
}

echo
if command -v unshare >/dev/null 2>&1 && unshare -rn true >/dev/null 2>&1; then
  echo "== ISOLATION: unshare -rn (network namespace dropped) =="
  BFLAG="--offline"
  runair unshare -rn
else
  echo "== ISOLATION FALLBACK: unshare -rn unavailable =="
  echo "   NOTE: running with --airgap + BCTX_OFFLINE=1 instead of a network"
  echo "   namespace. --airgap hard-disables acquisition and the TUI/geo/map"
  echo "   paths never dial, so the air-gap guarantee still holds; the namespace"
  echo "   just cannot be physically removed here."
  export BCTX_OFFLINE=1
  BFLAG="--airgap"
  runair env BCTX_OFFLINE=1
fi

echo
echo "== SUMMARY: PASS=$PASS FAIL=$FAIL  (workspace $WORK) =="
rm -rf "$WORK"
[ $FAIL -eq 0 ]
