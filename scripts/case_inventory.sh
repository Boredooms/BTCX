#!/usr/bin/env bash
# Print a compact inventory of every case's populated data so we can see which
# cases have transactions / graph / network observations / alerts.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2
BIN=./bin/bctx

printf '%-16s %6s %8s %8s %8s %7s\n' CASE TX WALLETS EDGES NETRECS ALERTS
for c in $("$BIN" case list 2>/dev/null | awk 'NR>1{print $1}'); do
  "$BIN" case open "$c" >/dev/null 2>&1
  "$BIN" --json status 2>/dev/null > /tmp/bctx-inv.json
  CASE="$c" python3 - <<'PY'
import os, json
c = os.environ["CASE"]
try:
    d = json.load(open("/tmp/bctx-inv.json"))
    print("%-16s %6s %8s %8s %8s %7s" % (c, d.get("transactions"), d.get("wallets"),
          d.get("edges"), d.get("network_recs"), d.get("alerts")))
except Exception as e:
    print("%-16s (err %s)" % (c, e))
PY
done
