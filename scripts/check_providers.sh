#!/usr/bin/env bash
set -u
check() {
  printf '%-50s ' "$1"
  curl -4 -s -o /dev/null -w 'HTTP %{http_code} in %{time_total}s\n' --max-time 15 "$1" || echo FAIL
}
check "https://mempool.space/api/blocks/tip/height"
check "https://blockstream.info/api/blocks/tip/height"
check "https://blockchain.info/latestblock"
echo "--- sample mempool tip ---"
curl -4 -s --max-time 15 "https://mempool.space/api/blocks/tip/height"; echo
echo "--- sample mempool block by height 800000 (hash) ---"
curl -4 -s --max-time 15 "https://mempool.space/api/block-height/800000"; echo
