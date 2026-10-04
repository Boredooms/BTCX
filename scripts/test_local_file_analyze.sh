#!/usr/bin/env bash
# Manual end-to-end check of the new local-file analyze mode:
#   bctx analyze wallet <addr> --file <dataset>
# Builds a tiny NDJSON dataset for a wallet, imports+analyzes OFFLINE, and also
# exercises the error paths (missing file, no-records-for-wallet).
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2
BIN=./bin/bctx
W="1LocalFileWalletTestAddrxxxxxxxxxxx"
OTHER="1OtherWalletxxxxxxxxxxxxxxxxxxxxxx"
DIR=/tmp/bctx-localfile
mkdir -p "$DIR"

# Bulk dataset: records for the target wallet AND another wallet.
cat > "$DIR/bulk.ndjson" <<EOF
{"txid":"$(printf 'a%.0s' {1..64})","timestamp":"2026-10-01T10:00:00Z","fee_btc":0.00001,"inputs":[{"address":"1SenderLocalAAAAAAAAAAAAAAAAAAAAAA","amount_btc":2.0}],"outputs":[{"address":"$W","amount_btc":1.99}]}
{"txid":"$(printf 'b%.0s' {1..64})","timestamp":"2026-10-01T11:00:00Z","fee_btc":0.00001,"inputs":[{"address":"$W","amount_btc":1.99}],"outputs":[{"address":"1DestLocalBBBBBBBBBBBBBBBBBBBBBBBB","amount_btc":1.98}]}
{"txid":"$(printf 'c%.0s' {1..64})","timestamp":"2026-10-01T12:00:00Z","fee_btc":0.00001,"inputs":[{"address":"$OTHER","amount_btc":5.0}],"outputs":[{"address":"1UnrelatedCCCCCCCCCCCCCCCCCCCCCCCC","amount_btc":4.99}]}
EOF

rm -rf "$HOME/.bctx/cases/localfile-test"
$BIN case create localfile-test >/dev/null 2>&1
$BIN case open localfile-test >/dev/null 2>&1

echo "================= HAPPY PATH (offline, --file) ================="
$BIN --offline analyze wallet "$W" --file "$DIR/bulk.ndjson" 2>&1

echo
echo "================= ERROR: missing file ================="
$BIN --offline analyze wallet "$W" --file /no/such/path/data.csv 2>&1

echo
echo "================= ERROR: no records for wallet ================="
$BIN --offline analyze wallet "1WalletNotInDatasetxxxxxxxxxxxxxxx" --file "$DIR/bulk.ndjson" 2>&1

echo
echo "================= REGRESSION: no --file (existing behavior) ================="
$BIN --offline analyze wallet "$W" 2>&1 | head -8
