#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: install-installer.sh [--bin-dir <dir>] [--from-dist-only]

Installs k-playbook-installer without requiring Go on the user machine.

The script mirrors all supported release artifacts to bin/, installs the
repo-local launcher wrapper, and links the global command to that wrapper.
For each platform binary it first looks under dist/ and otherwise downloads
the latest release.

Options:
  --bin-dir <dir>       Install directory. Default: ~/.local/bin.
  --from-dist-only      Do not download; fail if dist/ has no matching binary.
  -h, --help            Show this help.

Environment:
  INSTALL_BIN                    Alternative install directory.
  K_PLAYBOOK_RELEASE_BASE_URL    Release download base URL.

Example:
  ./scripts/install-installer.sh
  k-playbook-installer
USAGE
}

log() {
  printf '%s\n' "$*" >&2
}

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PLAYBOOK_REPO="$(cd "$SCRIPT_DIR/.." && pwd -P)"

BINARY="k-playbook-installer"
DIST_DIR="$PLAYBOOK_REPO/dist"
BIN_DIR="$PLAYBOOK_REPO/bin"
WRAPPER_TEMPLATE="$PLAYBOOK_REPO/scripts/templates/k-playbook-installer-wrapper.sh"
INSTALL_BIN="${INSTALL_BIN:-$HOME/.local/bin}"
RELEASE_BASE_URL="${K_PLAYBOOK_RELEASE_BASE_URL:-https://github.com/kascada/k-playbook/releases/latest/download}"
FROM_DIST_ONLY=0
RELEASE_TARGETS=(linux-amd64 linux-arm64 darwin-amd64 darwin-arm64)

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bin-dir)
      [[ $# -ge 2 ]] || die "Missing value for --bin-dir."
      INSTALL_BIN="$2"
      shift 2
      ;;
    --from-dist-only)
      FROM_DIST_ONLY=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "Unknown argument: $1"
      ;;
  esac
done

INSTALL_BIN="${INSTALL_BIN/#\~/$HOME}"

download() {
  local url="$1"
  local dest="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fL --retry 3 -o "$dest" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$dest" "$url"
  else
    die "Need curl or wget to download $url. Alternatively build to bin/ or copy a matching release artifact to dist/."
  fi
}

[[ -f "$WRAPPER_TEMPLATE" ]] || die "Missing wrapper template: $WRAPPER_TEMPLATE"
tmp_dir=""

cleanup() {
  if [[ -n "$tmp_dir" ]]; then
    rm -rf "$tmp_dir"
  fi
}
trap cleanup EXIT

mkdir -p "$BIN_DIR" "$INSTALL_BIN"
install -m 0755 "$WRAPPER_TEMPLATE" "$BIN_DIR/$BINARY"
log "Installiert: $BIN_DIR/$BINARY"

for target in "${RELEASE_TARGETS[@]}"; do
  asset="$BINARY-$target"
  local_asset="$DIST_DIR/$asset"
  if [[ -f "$local_asset" ]]; then
    source_binary="$local_asset"
    log "Nutze lokales Release-Artefakt: $local_asset"
  else
    [[ "$FROM_DIST_ONLY" -eq 0 ]] || die "Missing local release artifact: $local_asset"
    if [[ -z "$tmp_dir" ]]; then
      tmp_dir="$(mktemp -d)"
    fi
    source_binary="$tmp_dir/$asset"
    url="$RELEASE_BASE_URL/$asset"
    log "Lade Release-Binary: $url"
    download "$url" "$source_binary" || die "Download fehlgeschlagen. Stelle sicher, dass das Release-Asset existiert: $asset."
  fi
  install -m 0755 "$source_binary" "$BIN_DIR/$asset"
  log "Installiert: $BIN_DIR/$asset"
done

ln -sfn "$BIN_DIR/$BINARY" "$INSTALL_BIN/$BINARY"
log "Verlinkt: $INSTALL_BIN/$BINARY -> $BIN_DIR/$BINARY"

case ":$PATH:" in
  *":$INSTALL_BIN:"*)
    log "PATH OK: $INSTALL_BIN ist im PATH."
    ;;
  *)
    log "Hinweis: $INSTALL_BIN ist nicht im PATH."
    log "Fuege z. B. diese Zeile zu deinem Shell-Profil hinzu:"
    log "  export PATH=\"$INSTALL_BIN:\$PATH\""
    log "Oder starte direkt:"
    log "  $INSTALL_BIN/$BINARY"
    ;;
esac
