#!/usr/bin/env bash
# BTCX installer — downloads the latest (or a pinned) Linux release, verifies
# its SHA-256 checksum, and installs the `bctx`/`btcx` binary onto your PATH.
#
# Usage (Linux):
#   curl -fsSL https://raw.githubusercontent.com/Boredooms/BTCX/main/install.sh | sh
#   # or pin a version:
#   curl -fsSL https://raw.githubusercontent.com/Boredooms/BTCX/main/install.sh | VERSION=v0.1.0 sh
#
# Environment overrides:
#   VERSION   release tag to install (default: latest)
#   PREFIX    install dir (default: /usr/local/bin if writable/sudo, else ~/.local/bin)
#   NO_SUDO=1 never use sudo (fall back to ~/.local/bin)
#
# The script is offline-after-install: it only uses the network to DOWNLOAD the
# release. BTCX itself runs fully offline once installed.
set -eu

REPO="Boredooms/BTCX"
BIN_NAMES="bctx btcx"
VERSION="${VERSION:-latest}"

say()  { printf '==> %s\n' "$*"; }
err()  { printf 'error: %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

# ---- platform check --------------------------------------------------------
OS="$(uname -s)"
ARCH="$(uname -m)"
[ "$OS" = "Linux" ] || err "this installer supports Linux only (got $OS). Build from source: go build -o bctx ./apps/bctx"
case "$ARCH" in
  x86_64|amd64) GOARCH="amd64" ;;
  *) err "unsupported architecture $ARCH (only linux/amd64 is released; build from source for others)";;
esac

# ---- downloader ------------------------------------------------------------
dl() { # dl <url> <outfile>
  if have curl; then curl -fsSL "$1" -o "$2"
  elif have wget; then wget -qO "$2" "$1"
  else err "need curl or wget to download releases"
  fi
}

API="https://api.github.com/repos/${REPO}/releases"
if [ "$VERSION" = "latest" ]; then
  say "resolving latest release of ${REPO} ..."
  TMPJSON="$(mktemp)"
  dl "${API}/latest" "$TMPJSON"
  VERSION="$(grep -oE '"tag_name"[ ]*:[ ]*"[^"]+"' "$TMPJSON" | head -1 | sed -E 's/.*"([^"]+)"$/\1/')"
  rm -f "$TMPJSON"
  [ -n "$VERSION" ] || err "could not resolve the latest release tag"
fi
say "installing BTCX ${VERSION} (linux/${GOARCH})"

TARBALL="bctx_${VERSION}_linux_${GOARCH}.tar.gz"
BASE="https://github.com/${REPO}/releases/download/${VERSION}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

say "downloading ${TARBALL} ..."
dl "${BASE}/${TARBALL}"        "${WORK}/${TARBALL}"
dl "${BASE}/${TARBALL}.sha256" "${WORK}/${TARBALL}.sha256" || true

# ---- verify checksum -------------------------------------------------------
if [ -s "${WORK}/${TARBALL}.sha256" ] && have sha256sum; then
  say "verifying SHA-256 checksum ..."
  ( cd "$WORK" && sha256sum -c "${TARBALL}.sha256" ) || err "checksum verification FAILED — refusing to install"
else
  say "warning: checksum not verified (sha256 file or sha256sum unavailable)"
fi

say "extracting ..."
tar -xzf "${WORK}/${TARBALL}" -C "$WORK"
[ -f "${WORK}/bctx" ] || err "release tarball did not contain the bctx binary"
chmod +x "${WORK}/bctx"

# ---- choose install dir ----------------------------------------------------
choose_dir() {
  if [ -n "${PREFIX:-}" ]; then printf '%s\n' "$PREFIX"; return; fi
  if [ "${NO_SUDO:-0}" != "1" ]; then
    if [ -w /usr/local/bin ] 2>/dev/null; then printf '/usr/local/bin\n'; return; fi
    if have sudo; then printf '/usr/local/bin\n'; return; fi
  fi
  mkdir -p "$HOME/.local/bin" 2>/dev/null || true
  printf '%s/.local/bin\n' "$HOME"
}
DEST="$(choose_dir)"
mkdir -p "$DEST" 2>/dev/null || true

install_one() { # install_one <name>
  target="${DEST}/$1"
  if cp "${WORK}/bctx" "$target" 2>/dev/null; then
    chmod +x "$target"
  elif [ "${NO_SUDO:-0}" != "1" ] && have sudo; then
    sudo cp "${WORK}/bctx" "$target" && sudo chmod +x "$target"
  else
    err "cannot write $target (try PREFIX=\$HOME/.local/bin, or run with sudo)"
  fi
  say "installed $target"
}
for n in $BIN_NAMES; do install_one "$n"; done

# ---- PATH hint -------------------------------------------------------------
case ":$PATH:" in
  *":$DEST:"*) : ;;
  *)
    echo
    say "NOTE: $DEST is not on your PATH. Add to your shell rc:"
    printf '      export PATH="%s:$PATH"\n' "$DEST"
    ;;
esac

echo
say "done. Verify with:"
echo "    bctx version"
echo "    bctx doctor"
echo "    btcx            # launch the interactive TUI"
