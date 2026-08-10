#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PLAYBOOK_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
# Die Matrix liegt neben diesem Skript, damit beide zusammen verschoben werden
# koennen und kein Pfad ins uebergeordnete Verzeichnis noetig ist.
TOOL_MATRIX_FILE="${K_SECURITY_TOOLS_MATRIX:-$SCRIPT_DIR/security-tools.tsv}"

# Tool installation is host/user-local by policy. Do not install into project venvs.
REQUIRED_TOOLS=()
OPTIONAL_TOOLS=()
STATUS_TOOLS=()
declare -A TOOL_REQUIRED=()
declare -A TOOL_INSTALLABLE=()
declare -A TOOL_ROLE=()
declare -A TOOL_DOCKER_IMAGE=()
declare -A TOOL_VERSION_ARGS=()

INSTALL_SPEC=""
JSON_OUTPUT=0
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
  cat <<USAGE
Usage: install-security-tools.sh [--preflight]
       install-security-tools.sh --install <missing|required|all|tool> [--method auto|native|docker|pipx|venv] [--yes]

Host-local security-tool preflight and installer for k-playbook.
No project files are written.

Required tools:
  $(join_by ', ' "${REQUIRED_TOOLS[@]}")

Tool matrix:
  $TOOL_MATRIX_FILE

Options:
  --preflight          Show tool status only. This is the default.
  --json               Print the tool status as JSON and exit. Read-only.
  --install <target>   Install target: missing, required, all, or one tool name.
  --method <method>    auto, native, docker, pipx, or venv. Default: auto.
  --include-optional   Accepted for old command lines; currently no extra tools.
  --prefix <dir>       User-local prefix for native binaries. Default: ~/.local.
  --bin-dir <dir>      Binary directory. Default: first PATH-visible of ~/.opencode/bin or ~/.local/bin, else <prefix>/bin.
  --venv <dir>         Dedicated pip-audit tool venv path for --method venv/auto fallback.
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

join_by() {
  local delimiter first item
  delimiter="$1"
  shift
  first=1
  for item in "$@"; do
    if [[ "$first" -eq 1 ]]; then
      first=0
    else
      printf '%s' "$delimiter"
    fi
    printf '%s' "$item"
  done
}

load_tool_matrix() {
  local name required installable role docker_image version_args

  [[ -f "$TOOL_MATRIX_FILE" ]] || die "Security-Tool-Matrix fehlt: $TOOL_MATRIX_FILE"

  while IFS=$'\t' read -r name required installable role docker_image version_args; do
    [[ -z "${name:-}" ]] && continue
    [[ "$name" == \#* ]] && continue
    [[ "$name" == "name" ]] && continue

    case "$required" in
      true|false) ;;
      *) die "Ungueltiger required-Wert fuer $name in $TOOL_MATRIX_FILE: $required" ;;
    esac
    case "$installable" in
      true|false) ;;
      *) die "Ungueltiger installable-Wert fuer $name in $TOOL_MATRIX_FILE: $installable" ;;
    esac

    STATUS_TOOLS+=("$name")
    TOOL_REQUIRED["$name"]="$required"
    TOOL_INSTALLABLE["$name"]="$installable"
    TOOL_ROLE["$name"]="$role"
    TOOL_DOCKER_IMAGE["$name"]="$docker_image"
    TOOL_VERSION_ARGS["$name"]="$version_args"

    if [[ "$required" == "true" ]]; then
      REQUIRED_TOOLS+=("$name")
    else
      OPTIONAL_TOOLS+=("$name")
    fi
  done < "$TOOL_MATRIX_FILE"

  [[ "${#REQUIRED_TOOLS[@]}" -gt 0 ]] || die "Security-Tool-Matrix enthaelt keine Pflicht-Tools: $TOOL_MATRIX_FILE"
}

has_cmd() {
  command -v "$1" >/dev/null 2>&1
}

ensure_no_active_project_venv() {
  if [[ -n "${VIRTUAL_ENV:-}" ]]; then
    die "Ein Python-venv ist aktiv ($VIRTUAL_ENV). Deaktiviere es zuerst mit 'deactivate'. Dieses Skript installiert nur host-/user-lokale Tools, nie in ein Projekt-venv."
  fi
}

ensure_no_project_venv_in_path() {
  local entry
  IFS=':' read -r -a path_entries <<< "$PATH"
  for entry in "${path_entries[@]}"; do
    [[ -z "$entry" ]] && continue
    if is_project_venv_path "$entry"; then
      die "PATH enthaelt ein typisches Projekt-venv ($entry). Entferne es zuerst aus PATH bzw. fuehre 'deactivate' aus, damit der Preflight nur host-/user-lokale Tools bewertet."
    fi
  done
}

is_project_venv_path() {
  local path
  path="${1%/}"
  case "$path" in
    .venv|.venv/*|venv|venv/*|env|env/*|*/.venv|*/.venv/*|*/venv|*/venv/*|*/env|*/env/*)
      return 0
      ;;
  esac
  return 1
}

ensure_host_tool_scope() {
  ensure_no_active_project_venv
  ensure_no_project_venv_in_path
  if is_project_venv_path "$BIN_DIR"; then
    die "Installationsziel liegt in einem typischen Projekt-venv: $BIN_DIR. Nutze ein user-lokales Bin-Verzeichnis wie ~/.opencode/bin oder ~/.local/bin."
  fi
  if is_project_venv_path "$VENV_DIR"; then
    die "pip-audit Tool-venv darf kein Projekt-venv sein: $VENV_DIR. Nutze pipx oder ein dediziertes Tool-venv unter ~/.local/share/k-playbook/."
  fi
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
  local tool output args arg
  tool="$1"

  IFS=',' read -r -a args <<< "${TOOL_VERSION_ARGS[$tool]:---version}"
  for arg in "${args[@]}"; do
    [[ -z "$arg" ]] && continue
    output="$("$tool" "$arg" 2>/dev/null || true)"
    [[ -n "$output" ]] && break
  done

  if [[ -z "$output" ]]; then
    output="version unknown"
  fi
  printf '%s' "${output%%$'\n'*}"
}

tool_role() {
  printf '%s' "${TOOL_ROLE[$1]:--}"
}

docker_image() {
  printf '%s' "${TOOL_DOCKER_IMAGE[$1]:--}"
}

all_tools_for_status() {
  printf '%s\n' "${STATUS_TOOLS[@]}"
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
  [[ -n "${TOOL_REQUIRED[$tool]+set}" ]]
}

is_installable_tool() {
  local tool
  tool="$1"
  [[ "${TOOL_INSTALLABLE[$tool]:-false}" == "true" ]]
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
  ensure_host_tool_scope
  missing_required="$(missing_required_count)"

  printf 'Security-Tools - Preflight\n'
  printf '%s\n' '--------------------------'
  printf 'Installation: %s\n' "$PLAYBOOK_DIR"
  printf 'Toolliste:  %s\n' "$TOOL_MATRIX_FILE"
  printf 'Bin dir:    %s\n' "$BIN_DIR"
  printf 'pip-audit:  %s (dediziertes Tool-venv, nicht Projekt-venv)\n' "$VENV_DIR"
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

# json_escape maskiert die Zeichen, die in einem JSON-String nicht roh stehen
# duerfen. Versionsausgaben und Pfade kommen von fremden Programmen, also wird
# hier nichts vorausgesetzt.
json_escape() {
  local text="$1"
  text="${text//\\/\\\\}"
  text="${text//\"/\\\"}"
  text="${text//$'\t'/\\t}"
  text="${text//$'\r'/}"
  text="${text//$'\n'/\\n}"
  printf '%s' "$text"
}

# print_preflight_json gibt denselben Zustand wie print_preflight aus, nur
# maschinenlesbar. Die GUI rendert daraus ihre Tabelle.
print_preflight_json() {
  local tool status version path image missing_required first
  ensure_host_tool_scope
  missing_required="$(missing_required_count)"

  printf '{\n'
  printf '  "playbookDir": "%s",\n' "$(json_escape "$PLAYBOOK_DIR")"
  printf '  "toolMatrix": "%s",\n' "$(json_escape "$TOOL_MATRIX_FILE")"
  printf '  "binDir": "%s",\n' "$(json_escape "$BIN_DIR")"
  printf '  "venvDir": "%s",\n' "$(json_escape "$VENV_DIR")"
  printf '  "missingRequired": %s,\n' "$missing_required"
  # Absoluter Pfad: der Befehl soll sich kopieren und von ueberall ausfuehren
  # lassen, unabhaengig vom Arbeitsverzeichnis des Aufrufs.
  printf '  "installCommand": "%s",\n' \
    "$(json_escape "bash \"$SCRIPT_DIR/$(basename "${BASH_SOURCE[0]}")\" --install missing --method auto")"
  printf '  "tools": [\n'

  first=1
  while IFS= read -r tool; do
    [[ -z "$tool" ]] && continue
    if has_cmd "$tool"; then
      status="ok"
      version="$(tool_version "$tool")"
      path="$(command -v "$tool")"
    else
      status="missing"
      version=""
      path=""
    fi
    image="$(docker_image "$tool")"
    [[ "$image" == "-" ]] && image=""

    [[ "$first" -eq 0 ]] && printf ',\n'
    first=0
    printf '    {"name": "%s", "required": %s, "status": "%s", "version": "%s", "path": "%s", "role": "%s", "dockerImage": "%s"}' \
      "$(json_escape "$tool")" \
      "$(is_required_tool "$tool" && printf 'true' || printf 'false')" \
      "$status" \
      "$(json_escape "$version")" \
      "$(json_escape "$path")" \
      "$(json_escape "$(tool_role "$tool")")" \
      "$(json_escape "$image")"
  done < <(all_tools_for_status)

  printf '\n  ]\n'
  printf '}\n'
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
      die "pipx is not installed. Use --method venv for the dedicated k-playbook tool venv, or install pipx first."
    fi
  fi

  has_cmd python3 || die "python3 is required for pip-audit tool-venv install."
  log "Installing pip-audit into dedicated tool venv: $VENV_DIR"

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
          log "Docker-Fallback fuer pip-audit ist nicht definiert; installiere pip-audit stattdessen in einem dedizierten Tool-venv."
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
        if is_installable_tool "$tool" && ! has_cmd "$tool"; then
          printf '%s\n' "$tool"
        fi
      done
      if [[ "$INCLUDE_OPTIONAL" -eq 1 ]]; then
        for tool in "${OPTIONAL_TOOLS[@]}"; do
          if is_installable_tool "$tool" && ! has_cmd "$tool"; then
            printf '%s\n' "$tool"
          fi
        done
      fi
      ;;
    required)
      installable_required_tools
      ;;
    optional)
      installable_optional_tools
      ;;
    all)
      installable_required_tools
      installable_optional_tools
      ;;
    *)
      if is_known_tool "$INSTALL_SPEC" && is_installable_tool "$INSTALL_SPEC"; then
        printf '%s\n' "$INSTALL_SPEC"
        return 0
      fi
      if is_known_tool "$INSTALL_SPEC"; then
        die "Tool ist in der Matrix enthalten, aber nicht installierbar: $INSTALL_SPEC"
      fi
      die "Unknown --install target: $INSTALL_SPEC"
      ;;
  esac
}

validate_install_spec() {
  case "$INSTALL_SPEC" in
    missing|required|optional|all)
      return 0
      ;;
  esac

  if is_known_tool "$INSTALL_SPEC" && is_installable_tool "$INSTALL_SPEC"; then
    return 0
  fi
  if is_known_tool "$INSTALL_SPEC"; then
    die "Tool ist in der Matrix enthalten, aber nicht installierbar: $INSTALL_SPEC"
  fi
  die "Unknown --install target: $INSTALL_SPEC"
}

installable_optional_tools() {
  local tool
  for tool in "${OPTIONAL_TOOLS[@]}"; do
    is_installable_tool "$tool" && printf '%s\n' "$tool"
  done
}

installable_required_tools() {
  local tool
  for tool in "${REQUIRED_TOOLS[@]}"; do
    is_installable_tool "$tool" && printf '%s\n' "$tool"
  done
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --preflight)
        shift
        ;;
      --json)
        JSON_OUTPUT=1
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
  load_tool_matrix
  parse_args "$@"

  # --json ist rein lesend: es beschreibt den Zustand und installiert nichts.
  if [[ "$JSON_OUTPUT" -eq 1 ]]; then
    print_preflight_json
    exit 0
  fi

  print_preflight

  if [[ -z "$INSTALL_SPEC" ]]; then
    exit 0
  fi

  validate_install_spec

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
