#!/usr/bin/env bash
# Live demo of the --file local data source: create a case, hand analyze a
# local CSV, and show the happy path + the three error/offline guarantees.
# Fully offline. Safe to re-run.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2
BIN=./bin/bctx
go build -o "$BIN" ./apps/bctx || exit 1

W="1DemoFileWalletXXXXXXXXXXXXXXXXXX"
OTHER="1OtherDemoWalletXXXXXXXXXXXXXXXX"
D=/tmp/bctx-demo-file
mkdir -p "$D"

# A bulk CSV with the target wallet (receive + spend) AND an unrelated wallet.
cat > "$D/bulk.csv" <<EOF
txid,timestamp,fee_btc,input_address,input_amount,output_address,output_amount
$(printf 'a%.0s' {1..64}),2026-10-01T10:00:00Z,0.00001,1SenderDemoAAAAAAAAAAAAAAAAAAAAAA,2.00000000,$W,1.99000000
$(printf 'b%.0s' {1..64}),2026-10-01T11:00:00Z,0.00001,$W,1.99000000,1DestDemoBBBBBBBBBBBBBBBBBBBBBBBB,1.98000000
$(printf 'c%.0s' {1..64}),2026-10-01T12:00:00Z,0.00001,$OTHER,5.00000000,1UnrelDemoCCCCCCCCCCCCCCCCCCCCCCC,4.99000000
EOF

rm -rf "$HOME/.bctx/cases/demo-file"
$BIN case create demo-file >/dev/null 2>&1
$BIN case open demo-file  >/dev/null 2>&1

echo "############ 1) HAPPY PATH — analyze W from the local CSV, OFFLINE ############"
$BIN --offline analyze wallet "$W" --file "$D/bulk.csv"

echo
echo "############ 2) FULLY AIR-GAPPED (hard offline flag) ############"
$BIN --airgap analyze wallet "$W" --file "$D/bulk.csv" | sed -n '1,8p'

echo
echo "############ 3) MISSING FILE — clear error, NO network fallback ############"
$BIN --offline analyze wallet "$W" --file /no/such/data.csv

echo
echo "############ 4) WALLET NOT IN DATASET — clear error, NO network fallback ############"
$BIN --offline analyze wallet "1NotInDatasetZZZZZZZZZZZZZZZZZZZZ" --file "$D/bulk.csv" | tail -4

echo
echo "############ 5) --file WINS over --sync (local is authoritative, offline) ############"
$BIN analyze wallet "$W" --file "$D/bulk.csv" --sync 2>&1 | sed -n '1,3p'

echo
echo "############ 6) REGRESSION — existing behavior with no --file (unchanged) ############"
$BIN --offline analyze wallet "$W" | sed -n '1,6p'
