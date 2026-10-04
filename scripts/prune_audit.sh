#!/usr/bin/env bash
# Read-only. Classifies every dir as EMPTY-SCAFFOLD (only .gitkeep, no .go, no
# subdir with code) vs REAL (has .go files or non-trivial content). Deletes nothing.
set -u
cd /home/devara/btcx

echo "=============================================================="
echo " EMPTY SCAFFOLD DIRS (only .gitkeep, nothing else, no .go anywhere below)"
echo "=============================================================="
while IFS= read -r d; do
  [ -z "$d" ] && continue
  gocount=$(find "$d" -name '*.go' 2>/dev/null | wc -l)
  files=$(find "$d" -type f ! -name '.gitkeep' 2>/dev/null | wc -l)
  if [ "$gocount" -eq 0 ] && [ "$files" -eq 0 ]; then
    echo "PRUNE?  $d"
  fi
done < <(find . -type d \
   -not -path './.git/*' \
   -not -path './.phase*-artifacts/*' \
   -not -path './ml-lab/*' \
   -not -path './bin/*' \
   -not -path '*/.gitkeep' | sort)

echo
echo "=============================================================="
echo " REAL DIRS (contain .go files) — KEEP"
echo "=============================================================="
while IFS= read -r d; do
  [ -z "$d" ] && continue
  gocount=$(find "$d" -maxdepth 1 -name '*.go' 2>/dev/null | wc -l)
  if [ "$gocount" -gt 0 ]; then
    echo "KEEP    $d  ($gocount .go)"
  fi
done < <(find . -type d \
   -not -path './.git/*' \
   -not -path './.phase*-artifacts/*' \
   -not -path './ml-lab/*' \
   -not -path './bin/*' | sort)
