#!/usr/bin/env bash
# Prove the OFFLINE half end to end on real local data: analyze + report +
# export + verify a subject with the network hard-off (--offline), including the
# risk band and report bundle. Uses the seeded demo-godmode case.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx
go build -o bin/bctx ./apps/bctx || exit 1
BIN=./bin/bctx
$BIN case open demo-godmode >/dev/null 2>&1

# Pick the fan-out hub (a HIGH-risk subject) from the seed.
HUB=$(python3 - <<'PY'
import json
d = json.load(open("scripts/bigseed.json"))
for r in d:
    if len(r.get("outputs", []))==8:
        print(r["inputs"][0]["address"]); break
PY
)
echo "subject: $HUB"
PASS=0; FAIL=0
ok(){ echo "PASS: $1"; PASS=$((PASS+1)); }
bad(){ echo "FAIL: $1"; echo "  $2"; FAIL=$((FAIL+1)); }

echo "== offline analyze (risk band) =="
OUT=$($BIN --offline --json analyze wallet "$HUB" 2>&1); RC=$?
BAND=$(echo "$OUT" | grep -o '\"band\": \"[A-Z]*\"' | head -1 | grep -o '[A-Z]*')
SCORE=$(echo "$OUT" | grep -o '\"score\": [0-9]*' | head -1 | grep -o '[0-9]*')
[ $RC -eq 0 ] && [ -n "$BAND" ] && ok "offline analyze (risk $SCORE / $BAND)" || bad "analyze" "$OUT"

echo "== offline report generate json|md|html|pdf =="
for fmt in json md html pdf; do
  $BIN --offline report generate "$HUB" --format "$fmt" >/dev/null 2>&1 && ok "report $fmt" || bad "report $fmt" ""
done

echo "== offline export + verify =="
OUT=$($BIN --offline report export "$HUB" 2>&1); RC=$?
echo "$OUT" | tail -3
BUNDLE=$(echo "$OUT" | grep -oE '/[^ ]*/report' | head -1)
[ $RC -eq 0 ] && ok "export bundle" || bad "export" "$OUT"
if [ -n "$BUNDLE" ]; then
  $BIN --offline report verify "$BUNDLE" 2>&1 | grep -qiE 'ok|verified|pass' && ok "verify" || bad "verify" "$($BIN --offline report verify "$BUNDLE" 2>&1)"
fi

echo "== SUMMARY: PASS=$PASS FAIL=$FAIL =="
[ $FAIL -eq 0 ]
