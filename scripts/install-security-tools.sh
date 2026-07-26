#!/usr/bin/env bash
set -euo pipefail

REQUIRED_TOOLS=(gitleaks trufflehog pip-audit trivy syft grype)
OPTIONAL_TOOLS=()

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PLAYBOOK_REPO="$(cd "$SCRIPT_DIR/.." && pwd -P)"

INSTALL_SPEC=""
METHOD="auto"
INCLUDE_OPTIONAL=0
YES=0
DRY_RUN=0
PREFIX="${K_SECURITY_TOOLS_PREFIX:-$HOME/.local}"
VENV_DIR="${K_SECURITY_TOOLS_VENV:-$HOME/.local/share/k-playbook/security-tools/pip-audit-venv}"

default_bin_dir() {
  local candidate
  for candidate in "$HOME/.opencode/bin" "$HOME/.local/bin"; do
    case ":$PATH:" in
      *":$candidate:"*)
        printf '%s' "$candidate"
        return
        ;;
    esac
  done
  printf '%s' "$PREFIX/bin"
}

BIN_DIR="${K_SECURITY_TOOLS_BIN_DIR:-$(default_bin_dir)}"

usage() {
  cat <<'USAGE'
Usage: install-security-tools.sh [--preflight]
       install-security-tools.sh --install <missing|required|all|tool> [--method auto|native|docker|pipx|venv] [--yes]

Host-local security-tool preflight and installer for k-playbook.
No project files are written.

Required tools:
  gitleaks, trufflehog, pip-audit, trivy, syft, grype

Options:
  --preflight          Show tool status only. This is the default.
  --install <target>   Install target: missing, required, all, or one tool name.
  --method <method>    auto, native, docker, pipx, or venv. Default: auto.
  --include-optional   Accepted for old command lines; currently no extra tools.
  --prefix <dir>       User-local prefix for native binaries. Default: ~/.local.
  --bin-dir <dir>      Binary directory. Default: first PATH-visible of ~/.opencode/bin or ~/.local/bin, else <prefix>/bin.
  --venv <dir>         pip-audit venv path for --method venv/auto fallback.
  --yes                Do not ask for confirmation after printing the install plan.
  --dry-run            Print what would happen, but do not install.
  -h, --help           Show this help.

Examples:
  scripts/install-security-tools.sh --preflight
  scripts/install-security-tools.sh --install missing --method auto
  scripts/install-security-tools.sh --install all --yes
USAGE
}

log() {
  printf '%s\n' "$*" >&2
}

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

has_cmd() {
  command -v "$1" >/dev/null 2>&1
}

run_or_print() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf 'DRY-RUN: %q' "$1" >&2
    shift
    for arg in "$@"; do
      printf ' %q' "$arg" >&2
    done
    printf '\n' >&2
  else
    "$@"
  fi
}

tool_version() {
  local tool output
  tool="$1"

  case "$tool" in
    gitleaks)
      output="$(gitleaks version 2>/dev/null || gitleaks --version 2>/dev/null || true)"
      ;;
    trufflehog)
      output="$(trufflehog --version 2>/dev/null || true)"
      ;;
    pip-audit)
      output="$(pip-audit --version 2>/dev/null || true)"
      ;;
    trivy)
      output="$(trivy --version 2>/dev/null || true)"
      ;;
    syft)
      output="$(syft version 2>/dev/null || syft --version 2>/dev/null || true)"
      ;;
    grype)
      output="$(grype version 2>/dev/null || grype --version 2>/dev/null || true)"
      ;;
    docker)
      output="$(docker --version 2>/dev/null || true)"
      ;;
    *)
      output=""
      ;;
  esac

  if [[ -z "$output" ]]; then
    output="version unknown"
  fi
  printf '%s' "${output%%$'\n'*}"
}

tool_role() {
  case "$1" in
    gitleaks) printf 'secret scanning' ;;
    trufflehog) printf 'deep secret scanning' ;;
    pip-audit) printf 'Python dependency CVE' ;;
    trivy) printf 'filesystem/container/IaC CVE' ;;
    syft) printf 'SBOM generation' ;;
    grype) printf 'SBOM/dependency CVE' ;;
    *) printf '-' ;;
  esac
}

docker_image() {
  case "$1" in
    gitleaks) printf 'ghcr.io/gitleaks/gitleaks:latest' ;;
    trufflehog) printf 'trufflesecurity/trufflehog:latest' ;;
    trivy) printf 'aquasec/trivy:latest' ;;
    syft) printf 'anchore/syft:latest' ;;
    grype) printf 'anchore/grype:latest' ;;
    pip-audit) printf '-' ;;
    *) printf '-' ;;
  esac
}

all_tools_for_status() {
  printf '%s\n' "${REQUIRED_TOOLS[@]}" "${OPTIONAL_TOOLS[@]}"
}

is_required_tool() {
  local tool
  tool="$1"
  for item in "${REQUIRED_TOOLS[@]}"; do
    [[ "$item" == "$tool" ]] && return 0
  done
  return 1
}

is_known_tool() {
  local tool
  tool="$1"
  for item in "${REQUIRED_TOOLS[@]}" "${OPTIONAL_TOOLS[@]}"; do
    [[ "$item" == "$tool" ]] && return 0
  done
  return 1
}

missing_required_count() {
  local count tool
  count=0
  for tool in "${REQUIRED_TOOLS[@]}"; do
    if ! has_cmd "$tool"; then
      count=$((count + 1))
    fi
  done
  printf '%s' "$count"
}

print_preflight() {
  local tool kind status version path image missing_required
  missing_required="$(missing_required_count)"

  printf '/k-install-security-tools - Preflight\n'
  printf '%s\n' '------------------------------------'
  printf 'Repo:       %s\n' "$PLAYBOOK_REPO"
  printf 'Bin dir:    %s\n' "$BIN_DIR"
  printf 'pip-audit:  %s\n' "$VENV_DIR"
  if has_cmd docker; then
    printf 'Docker:     ok (%s)\n' "$(tool_version docker)"
  else
    printf 'Docker:     fehlt (Docker-Fallback nicht verfuegbar)\n'
  fi
  printf '\n'
  printf '%-12s %-9s %-8s %-34s %s\n' 'Tool' 'Pflicht' 'Status' 'Version/Pfad' 'Docker-Fallback'
  printf '%-12s %-9s %-8s %-34s %s\n' '----' '-------' '------' '------------' '---------------'

  while IFS= read -r tool; do
    [[ -z "$tool" ]] && continue
    if is_required_tool "$tool"; then
      kind="ja"
    else
      kind="optional"
    fi
    image="$(docker_image "$tool")"
    if has_cmd "$tool"; then
      status="ok"
      version="$(tool_version "$tool")"
      path="$(command -v "$tool")"
      printf '%-12s %-9s %-8s %-34s %s\n' "$tool" "$kind" "$status" "${version:0:33}" "$image"
      printf '%-12s %-9s %-8s %-34s %s\n' '' '' '' "$path" ''
    else
      status="fehlt"
      printf '%-12s %-9s %-8s %-34s %s\n' "$tool" "$kind" "$status" "$(tool_role "$tool")" "$image"
    fi
  done < <(all_tools_for_status)

  printf '\n'
  if [[ "$missing_required" -eq 0 ]]; then
    printf 'Status: Alle Pflicht-Tools sind installiert.\n'
  else
    printf 'Status: %s Pflicht-Tool(s) fehlen.\n' "$missing_required"
    printf '\n'
    printf 'Installationswege:\n'
    printf '  Native/user-local: bash "%s" --install missing --method auto\n' "$0"
    printf '  Docker-Fallback:   bash "%s" --install missing --method docker\n' "$0"
  fi
}

ensure_bin_dir() {
  if [[ "$DRY_RUN" -eq 0 ]]; then
    mkdir -p "$BIN_DIR"
  fi

  case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *)
      log "WARN: $BIN_DIR ist nicht in PATH. Installierte Tools sind erst nach PATH-Anpassung direkt aufrufbar."
      ;;
  esac
}

download_file() {
  local url dest
  url="$1"
  dest="$2"

  if has_cmd curl; then
    run_or_print curl -L --fail --show-error --output "$dest" "$url"
  elif has_cmd wget; then
    run_or_print wget -O "$dest" "$url"
  else
    die "curl oder wget ist fuer native Downloads erforderlich."
  fi
}

platform_key() {
  local os arch
  case "$(uname -s)" in
    Linux) os="linux" ;;
    Darwin) os="darwin" ;;
    *) die "Unsupported OS for native GitHub-release installs: $(uname -s)" ;;
  esac

  case "$(uname -m)" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) die "Unsupported architecture for native GitHub-release installs: $(uname -m)" ;;
  esac

  printf '%s %s' "$os" "$arch"
}

latest_asset() {
  local tool repo os arch
  tool="$1"
  repo="$2"
  read -r os arch < <(platform_key)

  python3 - "$repo" "$tool" "$os" "$arch" <<'PY'
import json
import re
import sys
import urllib.request

repo, tool, os_name, arch = sys.argv[1:]
url = f"https://api.github.com/repos/{repo}/releases/latest"
req = urllib.request.Request(
    url,
    headers={"Accept": "application/vnd.github+json", "User-Agent": "k-playbook"},
)
with urllib.request.urlopen(req, timeout=30) as response:
    data = json.load(response)

assets = data.get("assets", [])
tag = data.get("tag_name", "latest")

patterns = []
if tool == "gitleaks":
    gitleaks_arch = "x64" if arch == "amd64" else "arm64"
    patterns.append(rf"^gitleaks_.*_{os_name}_{gitleaks_arch}\.tar\.gz$")
elif tool == "trufflehog":
    patterns.append(rf"^trufflehog_.*_{os_name}_{arch}\.tar\.gz$")
elif tool == "trivy":
    if os_name != "linux":
        raise SystemExit("trivy native GitHub-release install is supported here only on Linux; use Homebrew or Docker on macOS")
    trivy_arch = "64bit" if arch == "amd64" else "ARM64"
    patterns.append(rf"^trivy_.*_Linux-{trivy_arch}\.tar\.gz$")
elif tool in {"syft", "grype"}:
    patterns.append(rf"^{tool}_.*_{os_name}_{arch}\.tar\.gz$")
else:
    raise SystemExit(f"no GitHub release mapping for {tool}")

for pattern in patterns:
    regex = re.compile(pattern)
    for asset in assets:
        name = asset.get("name", "")
        if regex.match(name):
            print(tag)
            print(asset["browser_download_url"])
            print(name)
            raise SystemExit(0)

names = ", ".join(asset.get("name", "") for asset in assets)
raise SystemExit(f"no matching release asset for {tool}; looked for {patterns}; available: {names}")
PY
}

github_repo_for_tool() {
  case "$1" in
    gitleaks) printf 'gitleaks/gitleaks' ;;
    trufflehog) printf 'trufflesecurity/trufflehog' ;;
    trivy) printf 'aquasecurity/trivy' ;;
    syft) printf 'anchore/syft' ;;
    grype) printf 'anchore/grype' ;;
    *) die "No GitHub repo mapping for $1" ;;
  esac
}

install_github_binary() {
  local tool repo tmp_dir asset_info tag url asset archive extract_dir candidate installed asset_lines
  tool="$1"
  repo="$(github_repo_for_tool "$tool")"

  has_cmd python3 || die "python3 is required to resolve latest GitHub releases."
  has_cmd tar || die "tar is required to extract release archives."

  asset_info="$(latest_asset "$tool" "$repo")"
  mapfile -t asset_lines <<< "$asset_info"
  tag="${asset_lines[0]:-}"
  url="${asset_lines[1]:-}"
  asset="${asset_lines[2]:-}"
  [[ -n "$tag" && -n "$url" && -n "$asset" ]] || die "Could not resolve release asset for $tool."

  log "Installing $tool $tag from $repo into $BIN_DIR"
  log "Asset: $asset"

  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf 'DRY-RUN: download %s\n' "$url" >&2
    printf 'DRY-RUN: install binary %s to %s/%s\n' "$tool" "$BIN_DIR" "$tool" >&2
    return
  fi

  tmp_dir="$(mktemp -d)"
  archive="$tmp_dir/$asset"
  extract_dir="$tmp_dir/extract"
  mkdir -p "$extract_dir"
  download_file "$url" "$archive"
  tar -xzf "$archive" -C "$extract_dir"

  installed=0
  shopt -s globstar nullglob
  for candidate in "$extract_dir"/**/"$tool"; do
    if [[ -f "$candidate" ]]; then
      install -m 0755 "$candidate" "$BIN_DIR/$tool"
      installed=1
      break
    fi
  done
  shopt -u globstar nullglob

  rm -rf "$tmp_dir"

  if [[ "$installed" -ne 1 ]]; then
    die "Release archive for $tool did not contain a $tool binary."
  fi
}

install_pip_audit() {
  local method
  method="$1"
  ensure_bin_dir

  if [[ "$method" == "auto" || "$method" == "pipx" ]]; then
    if has_cmd pipx; then
      log "Installing pip-audit with pipx"
      run_or_print pipx install --force pip-audit
      return
    fi
    if [[ "$method" == "pipx" ]]; then
      die "pipx is not installed. Use --method venv or install pipx first."
    fi
  fi

  has_cmd python3 || die "python3 is required for pip-audit venv install."
  log "Installing pip-audit into venv: $VENV_DIR"

  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf 'DRY-RUN: python3 -m venv %q\n' "$VENV_DIR" >&2
    printf 'DRY-RUN: %q -m pip install --upgrade pip pip-audit\n' "$VENV_DIR/bin/python" >&2
    printf 'DRY-RUN: ln -sfn %q %q\n' "$VENV_DIR/bin/pip-audit" "$BIN_DIR/pip-audit" >&2
    return
  fi

  python3 -m venv "$VENV_DIR"
  "$VENV_DIR/bin/python" -m pip install --upgrade pip pip-audit
  ln -sfn "$VENV_DIR/bin/pip-audit" "$BIN_DIR/pip-audit"
}

install_docker_image() {
  local tool image
  tool="$1"
  image="$(docker_image "$tool")"

  if [[ "$image" == "-" ]]; then
    log "Docker-Fallback fuer $tool ist nicht definiert; nutze pipx/venv/native."
    return 1
  fi
  has_cmd docker || die "Docker ist nicht installiert."
  log "Pulling Docker fallback image for $tool: $image"
  run_or_print docker pull "$image"
}

install_tool() {
  local tool method
  tool="$1"
  method="$2"

  case "$tool" in
    pip-audit)
      case "$method" in
        auto|native|pipx|venv) install_pip_audit "$method" ;;
        docker)
          log "Docker-Fallback fuer pip-audit ist nicht definiert; installiere pip-audit stattdessen in venv."
          install_pip_audit venv
          ;;
        *) die "Unknown install method for pip-audit: $method" ;;
      esac
      ;;
    gitleaks|trufflehog|trivy|syft|grype)
      case "$method" in
        auto|native)
          ensure_bin_dir
          install_github_binary "$tool"
          ;;
        docker)
          install_docker_image "$tool"
          ;;
        pipx|venv)
          die "$method is only supported for pip-audit."
          ;;
        *) die "Unknown install method: $method" ;;
      esac
      ;;
    *)
      die "Unknown tool: $tool"
      ;;
  esac
}

selected_tools() {
  local tool
  case "$INSTALL_SPEC" in
    missing)
      for tool in "${REQUIRED_TOOLS[@]}"; do
        has_cmd "$tool" || printf '%s\n' "$tool"
      done
      if [[ "$INCLUDE_OPTIONAL" -eq 1 ]]; then
        for tool in "${OPTIONAL_TOOLS[@]}"; do
          has_cmd "$tool" || printf '%s\n' "$tool"
        done
      fi
      ;;
    required)
      printf '%s\n' "${REQUIRED_TOOLS[@]}"
      ;;
    optional)
      return 0
      ;;
    all)
      printf '%s\n' "${REQUIRED_TOOLS[@]}" "${OPTIONAL_TOOLS[@]}"
      ;;
    gitleaks|trufflehog|pip-audit|trivy|syft|grype)
      printf '%s\n' "$INSTALL_SPEC"
      ;;
    *)
      die "Unknown --install target: $INSTALL_SPEC"
      ;;
  esac
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --preflight)
        shift
        ;;
      --install)
        INSTALL_SPEC="${2:-}"
        [[ -n "$INSTALL_SPEC" ]] || die "--install requires a target."
        shift 2
        ;;
      --method)
        METHOD="${2:-}"
        [[ -n "$METHOD" ]] || die "--method requires a value."
        shift 2
        ;;
      --include-optional)
        INCLUDE_OPTIONAL=1
        shift
        ;;
      --prefix)
        PREFIX="${2:-}"
        [[ -n "$PREFIX" ]] || die "--prefix requires a path."
        BIN_DIR="$PREFIX/bin"
        shift 2
        ;;
      --bin-dir)
        BIN_DIR="${2:-}"
        [[ -n "$BIN_DIR" ]] || die "--bin-dir requires a path."
        shift 2
        ;;
      --venv)
        VENV_DIR="${2:-}"
        [[ -n "$VENV_DIR" ]] || die "--venv requires a path."
        shift 2
        ;;
      --yes)
        YES=1
        shift
        ;;
      --dry-run)
        DRY_RUN=1
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

  case "$METHOD" in
    auto|native|docker|pipx|venv) ;;
    *) die "Unknown --method value: $METHOD" ;;
  esac
}

main() {
  local tools tool count answer
  parse_args "$@"

  print_preflight

  if [[ -z "$INSTALL_SPEC" ]]; then
    exit 0
  fi

  mapfile -t tools < <(selected_tools)
  count="${#tools[@]}"

  if [[ "$count" -eq 0 ]]; then
    printf '\nNichts zu installieren fuer Auswahl: %s\n' "$INSTALL_SPEC"
    exit 0
  fi

  printf '\nInstallationsplan\n'
  printf '%s\n' '-----------------'
  printf 'Methode:  %s\n' "$METHOD"
  printf 'Bin dir:  %s\n' "$BIN_DIR"
  printf 'Tools:\n'
  for tool in "${tools[@]}"; do
    is_known_tool "$tool" || die "Unknown tool selected: $tool"
    printf '  - %s (%s)\n' "$tool" "$(tool_role "$tool")"
  done

  if [[ "$YES" -ne 1 && "$DRY_RUN" -ne 1 ]]; then
    printf '\nJetzt installieren? [y/N] '
    read -r answer
    case "$answer" in
      y|Y|yes|YES|ja|JA) ;;
      *)
        printf 'Abgebrochen. Es wurde nichts installiert.\n'
        exit 0
        ;;
    esac
  fi

  for tool in "${tools[@]}"; do
    install_tool "$tool" "$METHOD"
  done

  printf '\nFinaler Status\n'
  printf '%s\n' '--------------'
  print_preflight
}

main "$@"
