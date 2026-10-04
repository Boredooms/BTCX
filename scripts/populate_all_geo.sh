#!/usr/bin/env bash
# Populate EVERY case with network (geo) observations + backfill recent
# transaction timestamps, so Geo Activity / Geo Map / Network and the live
# transaction feed are populated end to end in every case. Fully offline.
#
# Idempotent: re-running overwrites the geopop observations rather than
# duplicating them. A case with zero transactions is skipped (nothing to
# geolocate).
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2

go build -o bin/geopop ./tools/geopop
./bin/geopop --all
