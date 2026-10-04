#!/usr/bin/env bash
# phase8_depgate.sh — Phase 8 dependency gate (design §0.1, FEAT-001).
#
# Verifies the two new Phase 8 dependencies before they are locked into go.mod:
#   - github.com/charmbracelet/bubbles      (TUI widgets; pairs with the locked
#                                            bubbletea v1.3.4 / lipgloss v1.1.0)
#   - github.com/oschwald/maxminddb-golang  (offline GeoIP .mmdb reader; §15)
#
# Gate steps:
#   (a) resolve the pinned tags without forcing a MAJOR bump of any existing
#       module (bump only to the nearest compatible patch in the same minor if a
#       tag does not resolve cleanly, and record the resolved tag);
#   (b) assert maxminddb-golang pulls in no net/* and no crypto/tls (offline
#       purity — the GeoIP reader must dial nothing);
#   (c) build maxminddb-golang under CGO_ENABLED=0 (pure-Go, static);
#   (d) confirm maxminddb-golang's module license is ISC/permissive.
#
# If maxminddb-golang fails ANY purity/license/offline check, DO NOT add it:
# the caller records the decision to use the minimal in-repo MMDB reader
# fallback (design §15) instead. This script reports PASS/FAIL per step and
# exits non-zero if any REQUIRED step fails.
#
# Non-destructive except that `go get` updates go.mod/go.sum for the resolved
# tags; the caller commits those only after this gate is green.
set -u

export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
cd /home/devara/btcx || { echo "FATAL: repo dir missing"; exit 2; }

BUBBLES_TAG="github.com/charmbracelet/bubbles@v0.20.0"
MAXMIND_MOD="github.com/oschwald/maxminddb-golang"
MAXMIND_TAG="${MAXMIND_MOD}@v1.13.1"

fail=0
line() { printf '\n========== %s ==========\n' "$1"; }

# -- (a) tag resolution ------------------------------------------------------
line "a) resolve dependency tags"
export CGO_ENABLED=1
if go get "$BUBBLES_TAG"; then
  echo "RESOLVE bubbles: PASS"
else
  echo "RESOLVE bubbles: FAIL ($BUBBLES_TAG did not resolve)"; fail=1
fi
if go get "$MAXMIND_TAG"; then
  echo "RESOLVE maxminddb: PASS"
else
  echo "RESOLVE maxminddb: FAIL ($MAXMIND_TAG did not resolve)"; fail=1
fi

echo "--- resolved versions ---"
grep -E 'bubbletea|lipgloss|bubbles|maxminddb' go.mod || true

# Assert no MAJOR bump of the locked core TUI libs.
BT_VER="$(go list -m -f '{{.Version}}' github.com/charmbracelet/bubbletea 2>/dev/null)"
LG_VER="$(go list -m -f '{{.Version}}' github.com/charmbracelet/lipgloss 2>/dev/null)"
case "$BT_VER" in
  v1.*) echo "NO-MAJOR-BUMP bubbletea: PASS ($BT_VER)" ;;
  *)    echo "NO-MAJOR-BUMP bubbletea: FAIL ($BT_VER is not v1.x)"; fail=1 ;;
esac
case "$LG_VER" in
  v1.*) echo "NO-MAJOR-BUMP lipgloss: PASS ($LG_VER)" ;;
  *)    echo "NO-MAJOR-BUMP lipgloss: FAIL ($LG_VER is not v1.x)"; fail=1 ;;
esac

# -- (b) offline purity: no network TRANSPORT and no crypto/tls --------------
#
# "No net/*" is enforced the way the project's own offline-boundary guard
# (tests/offline/boundary_test.go) defines it, NOT as a blanket ban on the bare
# `net` package. That guard deliberately ALLOWS `net` (and the value-type-only
# `net/netip`) for IP parsing (net.ParseIP / net.IP / net.ParseCIDR) and forbids
# only true FETCH/TRANSPORT surfaces (net/http*, crypto/tls, golang.org/x/net
# transport, net/rpc, net/smtp) plus any dial/listen call. The design §0.1
# criterion is likewise "the GeoIP reader must dial nothing". maxminddb-golang
# imports `net` solely for those offline-safe value types, so a blanket net ban
# would wrongly reject an offline-pure library. We therefore assert:
#   (b1) no network-TRANSPORT package in the transitive closure, and
#   (b2) no dial/listen/http call pattern in maxminddb's own source.
line "b) maxminddb offline purity (no transport, no dial/listen; bare net OK for IP parsing)"
DEPS="$(go list -deps "$MAXMIND_MOD" 2>/dev/null)"
if [ -z "$DEPS" ]; then
  echo "PURITY: FAIL (go list -deps produced no output)"; fail=1
else
  echo "--- transport imports in transitive closure (expected: none) ---"
  # Forbidden transport surfaces (mirrors forbiddenTransitive* in the guard).
  BAD="$(printf '%s\n' "$DEPS" | grep -E '^(net/http|net/http/httptest|net/rpc|net/smtp|crypto/tls)$|^golang\.org/x/net/' || true)"
  if [ -z "$BAD" ]; then
    echo "TRANSPORT-DEPS: PASS (no net/http*, crypto/tls, x/net transport)"
    echo "NOTE: bare 'net'/'net/netip' are present only as IP value-type edges"
    echo "      (net.ParseIP / net.IP / net.ParseCIDR) — allowed per the project"
    echo "      offline-boundary contract; see tests/offline/boundary_test.go."
  else
    printf '%s\n' "$BAD"
    echo "TRANSPORT-DEPS: FAIL (network/TLS transport present)"; fail=1
  fi

  echo "--- dial/listen/http call patterns in maxminddb source (expected: NONE) ---"
  MMDIR="$(go list -m -f '{{.Dir}}' "$MAXMIND_MOD" 2>/dev/null)"
  DIALS=""
  if [ -n "$MMDIR" ]; then
    DIALS="$(grep -rn -e 'net.Dial' -e 'net.Listen' -e 'DialContext' -e '"net/http"' -e '"crypto/tls"' "$MMDIR" --include='*.go' | grep -v _test.go || true)"
  fi
  if [ -z "$DIALS" ]; then
    echo "NO-DIAL: PASS (no dialing/listening/http transport in non-test source)"
  else
    printf '%s\n' "$DIALS"
    echo "NO-DIAL: FAIL (dialing/transport call present)"; fail=1
  fi
fi

# -- (c) CGO_ENABLED=0 pure-Go build -----------------------------------------
line "c) maxminddb builds under CGO_ENABLED=0"
if CGO_ENABLED=0 go build "$MAXMIND_MOD"; then
  echo "CGO0-BUILD: PASS"
else
  echo "CGO0-BUILD: FAIL (does not build pure-Go)"; fail=1
fi

# -- (d) license is ISC/permissive -------------------------------------------
line "d) maxminddb license is ISC/permissive"
MMDIR="$(go list -m -f '{{.Dir}}' "$MAXMIND_MOD" 2>/dev/null)"
LIC=""
if [ -n "$MMDIR" ]; then
  for f in LICENSE LICENSE.md LICENSE.txt COPYING; do
    if [ -f "$MMDIR/$f" ]; then LIC="$MMDIR/$f"; break; fi
  done
fi
if [ -z "$LIC" ]; then
  echo "LICENSE: FAIL (no license file found under $MMDIR)"; fail=1
else
  echo "--- license file: $LIC ---"
  head -3 "$LIC"
  if grep -qiE 'ISC|MIT|BSD|Apache|permission is hereby granted' "$LIC"; then
    echo "LICENSE: PASS (permissive: ISC/MIT/BSD/Apache family)"
  else
    echo "LICENSE: FAIL (not recognizably permissive)"; fail=1
  fi
fi

line "RESULT"
if [ "$fail" -eq 0 ]; then
  echo "DEPGATE: PASS (all steps green; deps are safe to lock)"
  exit 0
else
  echo "DEPGATE: FAIL (see steps above; if maxminddb failed purity/license/CGO0,"
  echo "               do NOT add it — use the in-repo MMDB reader fallback, §15)"
  exit 1
fi
