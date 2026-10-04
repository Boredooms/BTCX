#!/usr/bin/env bash
# Probe a few notable real mainnet addresses for tx volume + balance so we can
# pick a meaningful one to demo. Prints chain tx_count and funded/spent sats.
set -u
CANDIDATES=(
  "bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97"  # Bitfinex-era (339)
  "bc1qa5wkgaew2dkv56kfvj49j0av5nml45x9ek9hz6"                       # Binance cold (very large)
  "1P5ZEDWTKTFGxQjZphgWPQUpe554WKDfHQ"                               # Binance hot
  "3M219KR5vEneNb47ewrPfWyb5jQ2DjxRP6"                               # Binance cold 2
  "1FeexV6bAHb8ybZjqQMjJrcCrHGW9sb6uF"                               # 79k BTC "whale" (dormant, famous)
  "bc1ql49ydapnjafl5t2cp9zqpjwe6pdgmxy98859v2"                       # OKX / exchange-era
)
for a in "${CANDIDATES[@]}"; do
  j=$(curl -4 -s --max-time 20 "https://blockstream.info/api/address/${a}")
  printf '%s\n' "$j" | python3 -c '
import sys, json
try:
    d = json.load(sys.stdin)
    cs = d.get("chain_stats", {})
    funded = cs.get("funded_txo_sum", 0)
    spent = cs.get("spent_txo_sum", 0)
    bal = (funded - spent) / 1e8
    tx = cs.get("tx_count", "?")
    addr = d.get("address")
    print("  tx=%s  balance=%.4f BTC  addr=%s" % (tx, bal, addr))
except Exception as e:
    print("  (no data: %s)" % e)
'
done
