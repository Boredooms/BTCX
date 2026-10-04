#!/usr/bin/env bash
# Probe candidate real-world mainnet addresses for tx volume so we can pick a
# good one to demo wallet analysis / monitoring (needs real history for
# patterns). Prints chain tx count per candidate from blockstream.info.
set -u
CANDIDATES=(
  "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"   # Satoshi genesis coinbase address (huge, slow)
  "bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97"  # Bitfinex cold (very large)
  "3FupZp77ySr7jwoLYEJ9mwzJpvoNBXsBnE"   # Binance-era hot (active)
  "1dice8EMZmqKvrGE4Qc9bUFf9PX3xaYDp"    # SatoshiDice (classic, many small txs)
)
for a in "${CANDIDATES[@]}"; do
  j=$(curl -4 -s --max-time 20 "https://blockstream.info/api/address/${a}")
  n=$(printf '%s' "$j" | python3 -c 'import sys,json;
try:
 d=json.load(sys.stdin); print(d["chain_stats"]["tx_count"])
except Exception as e: print("?",e)' 2>/dev/null)
  echo "${a}  chain_tx_count=${n}"
done
