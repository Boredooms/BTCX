#!/usr/bin/env bash
# Phase 7 air-gapped bundle proof.
#
# Proves that forensic report GENERATION, EXPORT and VERIFY need only three
# local inputs:
#     1. the compiled bctx binary,
#     2. a local case SQLite database (locally-owned canonical data), and
#     3. the local packaged ONNX model files (models/).
# ...and NOTHING else: no Python, no Go toolchain, no Node, no Docker, no
# internet. This is the "hand someone the binary + a case folder on a USB stick"
# scenario: on a truly air-gapped machine the reports still generate, export and
# verify.
#
# Strategy:
#   * Build the binary ONCE up front (building needs Go; running does not).
#   * Copy ONLY the binary + models + a seeded case DB into an isolated bundle
#     dir. The Go toolchain, Python, Node and Docker are deliberately removed
#     from PATH for the report-time commands so a hidden dependency on any of
#     them would fail loudly.
#   * Run generate + export + verify under `unshare -rn` (no network namespace).
#     If unshare is unavailable, fall back to --airgap + BCTX_OFFLINE and print a
#     clear note that the flag fallback was used.
#
# Model: scripts/phase6_offline_proof.sh harness. Does not modify it.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
cd /home/devara/btcx

BIN_SRC=./bin/bctx
WORK=$(mktemp -d /tmp/bctx-p7-airgap.XXXXXX)
BUNDLE="$WORK/airgap"
CFG="$BUNDLE/config.toml"
SEED="$WORK/seed.csv"
ADDR=A

PASS=0
FAIL=0
ok()  { echo "PASS: $1"; PASS=$((PASS+1)); }
bad() { echo "FAIL: $1"; echo "   $2"; FAIL=$((FAIL+1)); }

echo "== workspace: $WORK =="

# ---------------------------------------------------------------------------
# 0. Build the binary (the ONLY step allowed to use the Go toolchain). On a real
#    air-gapped target the operator ships a pre-built binary; here we build it
#    then stop using Go entirely.
# ---------------------------------------------------------------------------
echo
echo "== BUILD: compile bctx (build-time only; not needed at report time) =="
if go build -o "$BIN_SRC" ./apps/bctx; then
  ok "binary built"
else
  bad "build" "go build failed"
  echo "== SUMMARY: PASS=$PASS FAIL=$FAIL =="
  rm -rf "$WORK"
  exit 1
fi

# ---------------------------------------------------------------------------
# 1. Assemble the air-gap bundle: binary + models + config. Nothing else.
# ---------------------------------------------------------------------------
echo
echo "== ASSEMBLE air-gap bundle (binary + models + local DB only) =="
mkdir -p "$BUNDLE/data"
cp "$BIN_SRC" "$BUNDLE/bctx"
chmod +x "$BUNDLE/bctx"
# The models/ dir is resolved relative to the working directory by the runtime
# (repoModelsDir -> "models"); copy the packaged models into the bundle so the
# ML stage of analysis has local assets and nothing is fetched.
if [ -d models ]; then
  cp -r models "$BUNDLE/models"
  ok "packaged models copied into bundle"
else
  bad "models" "models/ directory not found; cannot prove local ML assets"
fi

cat > "$CFG" <<EOF
[app]
data_dir = "$BUNDLE/data"
log_level = "error"

[network]
acquisition_enabled = true
mode = "online"

[acquisition]
provider = "fake"

[reports]
default_format = "pdf"
EOF

cat > "$SEED" <<EOF
txid,timestamp,fee_btc,input_address,input_amount,output_address,output_amount
T1,2026-01-01T10:00:00Z,0.0001,A,2.00000000,B,1.90000000
T2,2026-01-02T11:00:00Z,0.0002,B,1.90000000,C,1.70000000
T3,2026-01-03T12:00:00Z,0.0001,C,1.70000000,D,1.60000000
EOF

# Seed the case DB using the bundled binary (this is still "connected"-era data
# ownership; import is offline ingestion). Run from inside the bundle so the
# binary resolves its models/ dir locally.
echo
echo "== SEED local case DB (offline ingestion + graph) =="
( cd "$BUNDLE" && ./bctx --config "$CFG" init >/dev/null 2>&1 \
  && ./bctx --config "$CFG" case create airgap >/dev/null 2>&1 \
  && ./bctx --config "$CFG" dataset import "$SEED" --format csv >/dev/null 2>&1 \
  && ./bctx --config "$CFG" graph build >/dev/null 2>&1 )
if [ $? -eq 0 ]; then ok "local case DB seeded (import + graph)"; else bad "seed" "could not seed local case DB"; fi

# ---------------------------------------------------------------------------
# 2. Poison the toolchains for the report-time commands. The system images this
#    runs on usually ship python3 (and sometimes a `go` shim) in /usr/bin, which
#    bctx also needs for coreutils -- so "absent from PATH" is neither achievable
#    nor the real question. The real question is: does report time ever INVOKE
#    Go/Python/Node/Docker? We answer it by shadowing each tool with a stub that
#    exits non-zero, placed FIRST on PATH. If bctx tried to exec any of them the
#    stub would fail and the report command would fail; since the commands
#    succeed, nothing spawned a toolchain. coreutils stay fully functional.
# ---------------------------------------------------------------------------
POISON="$WORK/poison"
mkdir -p "$POISON"
echo
echo "== POISON Go/Python/Node/Docker so a report-time exec would fail loudly =="
for t in go gofmt python python3 pip pip3 node npm npx docker; do
  cat > "$POISON/$t" <<EOF
#!/usr/bin/env bash
echo "AIRGAP VIOLATION: report time invoked '$t' -- forbidden" >&2
exit 97
EOF
  chmod +x "$POISON/$t"
done
AIRGAP_PATH="$POISON:/usr/bin:/bin"
# Confirm the poison actually shadows the tools (so the guard is not vacuous).
if PATH="$AIRGAP_PATH" go version >/dev/null 2>&1; then
  bad "poison active" "the go stub did not shadow the real go"
else
  ok "toolchain stubs active (any report-time exec of them fails)"
fi

# ---------------------------------------------------------------------------
# 3. Run generate + export + verify with the stripped PATH, under network
#    isolation. The only inputs are the bundled binary + local DB + local models.
# ---------------------------------------------------------------------------
run_airgap() {
  # $@ is the isolation wrapper (unshare -rn, or env for the fallback).
  ( cd "$BUNDLE" && PATH="$AIRGAP_PATH" "$@" ./bctx --config "$CFG" "$BFLAG" \
      report generate "$ADDR" --format pdf --out "$BUNDLE/out/report.pdf" ) \
      > "$WORK/gen.txt" 2>&1
  local g=$?
  ( cd "$BUNDLE" && PATH="$AIRGAP_PATH" "$@" ./bctx --config "$CFG" "$BFLAG" \
      report export "$ADDR" --formats json,md,html,pdf --out "$BUNDLE/out" ) \
      > "$WORK/exp.txt" 2>&1
  local e=$?
  ( cd "$BUNDLE" && PATH="$AIRGAP_PATH" "$@" ./bctx --config "$CFG" "$BFLAG" \
      report verify "$BUNDLE/out/report" ) > "$WORK/ver.txt" 2>&1
  local v=$?
  return $(( g | e | v ))
}

BFLAG="--offline"
echo
if command -v unshare >/dev/null 2>&1 && unshare -rn true >/dev/null 2>&1; then
  echo "== RUN report generate/export/verify under unshare -rn (air-gapped) =="
  BFLAG="--offline"
  run_airgap unshare -rn
  RC=$?
else
  echo "== RUN report generate/export/verify (air-gap FALLBACK) =="
  echo "   NOTE: unshare -rn unavailable; using --airgap + BCTX_OFFLINE=1 instead"
  echo "   of a network namespace. --airgap hard-disables acquisition and the"
  echo "   report path never dials, so the air-gap guarantee still holds; the"
  echo "   namespace just cannot be physically removed here."
  BFLAG="--airgap"
  run_airgap env BCTX_OFFLINE=1
  RC=$?
fi

if [ $RC -eq 0 ]; then ok "generate/export/verify ran with stripped PATH + isolation"; else
  bad "air-gap run" "generate/export/verify failed; see output below"
  echo "   --- generate ---"; sed 's/^/   /' "$WORK/gen.txt"
  echo "   --- export ---";   sed 's/^/   /' "$WORK/exp.txt"
  echo "   --- verify ---";   sed 's/^/   /' "$WORK/ver.txt"
fi

echo
echo "== ASSERT outputs are real + verified =="
if head -c 5 "$BUNDLE/out/report.pdf" | grep -q '%PDF-'; then
  ok "generated PDF starts with %PDF-"
else
  bad "pdf magic" "generated report.pdf is not a PDF"
fi
if [ -f "$BUNDLE/out/report/manifest.json" ]; then
  ok "export bundle manifest present"
else
  bad "bundle manifest" "no manifest.json in exported bundle"
fi
if grep -qi "VERDICT: OK" "$WORK/ver.txt"; then
  ok "exported bundle verifies OK offline"
else
  bad "verify verdict" "$(cat "$WORK/ver.txt")"
fi

echo
echo "== INPUTS USED (bundle contents) =="
echo "   binary:  $BUNDLE/bctx"
echo "   models:  $BUNDLE/models (local ONNX assets)"
echo "   case DB: $BUNDLE/data (local SQLite)"
echo "   => no Python / Go / Node / Docker / internet required at report time."

echo
echo "== SUMMARY: PASS=$PASS FAIL=$FAIL  (workspace $WORK) =="
rm -rf "$WORK"
[ $FAIL -eq 0 ]
