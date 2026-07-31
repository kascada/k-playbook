#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: install-installer.sh [--bin-dir <dir>] [--from-dist-only]

Installs k-playbook-installer without requiring Go on the user machine.

The script first looks for a matching local release artifact under dist/, then
for an already built source binary under bin/, and otherwise downloads the latest release.

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
INSTALL_BIN="${INSTALL_BIN:-$HOME/.local/bin}"
RELEASE_BASE_URL="${K_PLAYBOOK_RELEASE_BASE_URL:-https://github.com/kascada/k-playbook/releases/latest/download}"
FROM_DIST_ONLY=0

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

detect_os() {
  case "$(uname -s)" in
    Linux) printf 'linux' ;;
    Darwin) printf 'darwin' ;;
    *) die "Unsupported OS: $(uname -s). Supported: Linux, macOS." ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) die "Unsupported architecture: $(uname -m). Supported: amd64, arm64." ;;
  esac
}

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

is_usable_local_binary() {
  local candidate="$1"

  [[ -f "$candidate" && -x "$candidate" ]] || return 1
  if "$candidate" --help >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

first_usable_local_binary() {
  local candidate

  for candidate in \
    "$PLAYBOOK_REPO/bin/$BINARY" \
    "$PLAYBOOK_REPO/installer/$BINARY"; do
    if is_usable_local_binary "$candidate"; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done

  return 1
}

os="$(detect_os)"
arch="$(detect_arch)"
asset="$BINARY-$os-$arch"
local_asset="$DIST_DIR/$asset"
tmp_dir=""

cleanup() {
  if [[ -n "$tmp_dir" ]]; then
    rm -rf "$tmp_dir"
  fi
}
trap cleanup EXIT

if [[ -f "$local_asset" ]]; then
  source_binary="$local_asset"
  log "Nutze lokales Release-Artefakt: $local_asset"
elif [[ "$FROM_DIST_ONLY" -eq 0 ]] && source_binary="$(first_usable_local_binary)"; then
  log "Nutze vorhandenes lokales Binary: $source_binary"
else
  [[ "$FROM_DIST_ONLY" -eq 0 ]] || die "Missing local release artifact: $local_asset"
  tmp_dir="$(mktemp -d)"
  source_binary="$tmp_dir/$asset"
  url="$RELEASE_BASE_URL/$asset"
  log "Lade Release-Binary: $url"
  download "$url" "$source_binary" || die "Download fehlgeschlagen. Stelle sicher, dass das Release-Asset existiert, oder kopiere ein passendes Binary nach bin/ oder dist/."
fi

mkdir -p "$INSTALL_BIN"
install -m 0755 "$source_binary" "$INSTALL_BIN/$BINARY"

log "Installiert: $INSTALL_BIN/$BINARY"

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
