#!/usr/bin/env bash
# Package the BCTX demo datasets as installable DEMO-EXAMPLE extensions under
# ~/.bctx/extensions/, so they appear in the Extensions / Knowledge Manager tab
# and can be loaded into a case with `bctx extension load <id>` — fully offline.
#
# Also seeds one example of each OTHER kind (knowledge-base, model-pack) so the
# manager shows the full taxonomy for a demo.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
cd /home/devara/btcx || exit 2

EXT="$HOME/.bctx/extensions"
mkdir -p "$EXT"

# Regenerate the real-wallet + suite datasets so the extensions ship a copy.
python3 scripts/gen_real_wallets.py /tmp/bctx-real >/dev/null 2>&1
python3 scripts/gen_demo_suite.py /tmp/bctx-suite >/dev/null 2>&1

make_demo() {
  # id, name, publisher, subject, dataset_src, geo_src, description
  local id="$1" name="$2" pub="$3" subj="$4" dsrc="$5" gsrc="$6" desc="$7"
  local dir="$EXT/$id"
  mkdir -p "$dir"
  cp "$dsrc" "$dir/data.ndjson"
  local geofield=""
  if [ -n "$gsrc" ] && [ -f "$gsrc" ]; then
    cp "$gsrc" "$dir/geo.ndjson"
    geofield='"geo":"geo.ndjson",'
  fi
  cat > "$dir/manifest.json" <<EOF
{
  "name": "$name",
  "version": "1.0.0",
  "kind": "demo-example",
  "publisher": "$pub",
  "description": "$desc",
  "dataset": "data.ndjson",
  $geofield
  "subject": "$subj"
}
EOF
  # Enable it by default so the demo shows active examples.
  echo "enabled by seed" > "$dir/.enabled"
  echo "packaged demo-example: $id"
}

# Real-address demo examples (LOW / MED / HIGH ladder).
make_demo real-low-whale "Dormant Whale (LOW)" "BCTX Demo" \
  "1FeexV6bAHb8ybZjqQMjJrcCrHGW9sb6uF" \
  /tmp/bctx-real/real-low.ndjson /tmp/bctx-real/real-low-geo.ndjson \
  "A real famous address with a single benign receive — reads LOW end to end."
make_demo real-med-hub "Active Hub (MEDIUM)" "BCTX Demo" \
  "12cbQLTFMXRnSzktFkuoG3eHoMeFtpTu3S" \
  /tmp/bctx-real/real-med.ndjson /tmp/bctx-real/real-med-geo.ndjson \
  "A real address with a receive and a modest fan-out — reads MEDIUM."
make_demo real-high-peeler "Peeling Spender (HIGH)" "BCTX Demo" \
  "bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97" \
  /tmp/bctx-real/real-high.ndjson /tmp/bctx-real/real-high-geo.ndjson \
  "A real address with a long peeling chain + fan-out bursts — reads HIGH/CRITICAL."

# A classic suite example (entity cluster) for variety.
make_demo cluster-cospend "Co-spend Cluster" "BCTX Demo" \
  "bc1qclustersubjectdddddddddddddddddddd0" \
  /tmp/bctx-suite/cluster.ndjson /tmp/bctx-suite/demo-cluster-geo.ndjson \
  "Three co-spending addresses — populates the Entity Cluster screen."

# One knowledge-base and one model-pack placeholder so the taxonomy is visible.
KB="$EXT/kb-typologies"
mkdir -p "$KB"
cat > "$KB/notes.md" <<'EOF'
# AML Typologies (sample knowledge base)
- Peeling chains: value forwarded hop-to-hop with small peels.
- Fan-out distribution: one input, many outputs in a short window.
- Co-spend clustering: common-input ownership heuristic.
EOF
cat > "$KB/manifest.json" <<'EOF'
{
  "name": "AML Typologies",
  "version": "1.0.0",
  "kind": "knowledge-base",
  "publisher": "Sample Ministry",
  "description": "Reference typology notes an analyst can consult offline.",
  "docs": "notes.md"
}
EOF
echo "enabled by seed" > "$KB/.enabled"
echo "packaged knowledge-base: kb-typologies"

echo
echo "Installed extensions:"
./bin/bctx extension list
