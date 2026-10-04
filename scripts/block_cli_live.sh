#!/usr/bin/env bash
# Live block CLI proof: acquire block 170 online, then inspect it OFFLINE.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx
BIN=./bin/bctx
CFG="$HOME/.config/bctx/config.toml"

# Fresh isolated case + online config.
WORK=$(mktemp -d /tmp/bctx-blockcli.XXXXXX)
cat > "$WORK/config.toml" <<EOF
[app]
data_dir = "$WORK/data"
log_level = "error"
[network]
acquisition_enabled = true
mode = "online"
[acquisition]
provider = "esplora"
EOF
$BIN --config "$WORK/config.toml" init >/dev/null 2>&1
$BIN --config "$WORK/config.toml" case create blk >/dev/null 2>&1

echo "== acquire block height 170 (ONLINE) =="
$BIN --config "$WORK/config.toml" block height 170 2>&1 | head -25

echo
echo "== inspect the block OFFLINE (must work, no network) =="
HASH=$($BIN --config "$WORK/config.toml" --json block height 170 2>&1 | grep -o '\"hash\": \"[0-9a-f]*\"' | head -1 | grep -o '[0-9a-f]\{64\}')
echo "hash=$HASH"
$BIN --config "$WORK/config.toml" --offline block inspect "$HASH" 2>&1 | head -15

echo
echo "== status counts =="
$BIN --config "$WORK/config.toml" --json status 2>&1 | grep -E 'transactions|wallets'

rm -rf "$WORK"
