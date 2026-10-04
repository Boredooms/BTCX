#!/usr/bin/env bash
# Read-only audit: finds candidate dead files/dirs for HUMAN review. Deletes nothing.
set -u
cd /home/devara/btcx

echo "== new Phase 7 docs present? =="
ls -1 docs/reporting-runtime.md docs/offline-architecture.md 2>&1

echo
echo "== directories that contain ONLY a .gitkeep (empty scaffolds, never filled) =="
while IFS= read -r d; do
  real=$(find "$d" -maxdepth 1 -type f ! -name '.gitkeep' | wc -l)
  keep=$(find "$d" -maxdepth 1 -name '.gitkeep' | wc -l)
  if [ "$keep" -ge 1 ] && [ "$real" -eq 0 ]; then
    sub=$(find "$d" -mindepth 1 -maxdepth 1 -type d | wc -l)
    echo "$d   (subdirs=$sub)"
  fi
done < <(find . -type d -not -path './.git/*' -not -path './.phase7-artifacts/*' -not -path './.worktrees/*')

echo
echo "== completely empty directories (no files at all) =="
find . -type d -empty -not -path './.git/*' -not -path './.phase7-artifacts/*' 2>/dev/null

echo
echo "== stray root artifacts (odd names) =="
ls -a | grep -E '^(C:|;|'\'')' || echo "none"

echo
echo "== binary / build artifacts tracked in tree =="
ls -1 bin/ 2>/dev/null || echo "no bin/"
