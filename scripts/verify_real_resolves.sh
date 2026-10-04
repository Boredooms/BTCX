#!/usr/bin/env bash
# Confirm each real-address subject RESOLVES in its case (the "no local match"
# fix): the wallet is in the wallets table and analyze returns a real result.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
cd /home/devara/btcx || exit 2
BIN=./bin/bctx

check() {
  local case_id="$1" addr="$2"
  $BIN case open "$case_id" >/dev/null 2>&1
  local out score txs
  out="$($BIN --offline analyze wallet "$addr" 2>&1)"
  score="$(printf '%s' "$out" | grep -oE 'Risk:[[:space:]]+[0-9]+' | grep -oE '[0-9]+' | head -1)"
  txs="$(printf '%s' "$out" | grep -oE 'Transactions:[[:space:]]+[0-9]+' | grep -oE '[0-9]+' | head -1)"
  if printf '%s' "$out" | grep -q 'INVESTIGATION COMPLETE'; then
    echo "RESOLVES  $case_id  $addr  (score=${score:-?} txs=${txs:-?})"
  else
    echo "NO-MATCH  $case_id  $addr"
    printf '%s\n' "$out" | head -3
  fi
}

check real-low  "1FeexV6bAHb8ybZjqQMjJrcCrHGW9sb6uF"
check real-med  "12cbQLTFMXRnSzktFkuoG3eHoMeFtpTu3S"
check real-high "bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97"
