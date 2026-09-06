#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PLAYBOOK_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
# Der Release-Weg und der Guard auf das Installationsziel liegen in einer
# gemeinsamen Bibliothek: install-base-tools.sh geht denselben Weg, und
# doppelter Code driftet nach der ersten einseitigen Änderung auseinander.
# Gesucht wird relativ zum Ort dieses Skripts, nicht zum Arbeitsverzeichnis.
# shellcheck source=lib/install-common.sh
source "$SCRIPT_DIR/lib/install-common.sh"
# Die Matrix liegt neben diesem Skript, damit beide zusammen verschoben werden
# können und kein Pfad ins übergeordnete Verzeichnis nötig ist.
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
declare -A TOOL_LANGUAGES=()
declare -A TOOL_INSTALL_METHOD=()
declare -A TOOL_INSTALL_REF=()
declare -A TOOL_ASSET_PATTERN=()

# Die Sprachen des Projekts, komma-getrennt. Leer heißt unbekannt: dann gilt ein
# sprachgebundenes Tool als optional, weil sich ohne diese Angabe nicht verlangen
# lässt, was vielleicht gar nicht gebraucht wird.
LANGUAGES=""

INSTALL_SPEC=""
JSON_OUTPUT=0
METHOD="auto"
INCLUDE_OPTIONAL=0
YES=0
DRY_RUN=0
PREFIX="${K_SECURITY_TOOLS_PREFIX:-$HOME/.local}"
# Wurzel der Tool-venvs, je pip-Tool eines darunter. Früher stand hier genau ein
# venv für pip-audit; mit mehreren pip-Tools braucht jedes seinen eigenen Ort,
# damit sich ihre Abhängigkeiten nicht in die Quere kommen.
VENV_ROOT="${K_SECURITY_TOOLS_VENV_ROOT:-$HOME/.local/share/k-playbook/security-tools}"

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

Required tools (a language-bound one counts only with a matching --languages):
  $(join_by ', ' "${REQUIRED_TOOLS[@]}")

Tool matrix:
  $TOOL_MATRIX_FILE

Options:
  --preflight          Show tool status only. This is the default.
  --json               Print the tool status as JSON and exit. Read-only.
  --install <target>   Install target: missing, required, all, or one tool name.
  --method <method>    auto, native, docker, pipx, or venv. Default: auto.
                       venv betrifft Python-CLI-Tools; andere Tools nutzen ihren
                       nativen Installationsweg.
  --include-optional   With --install missing, also install the optional tools that apply
                       to the selected languages, not only the required ones.
  --prefix <dir>       User-local prefix for native binaries. Default: ~/.local.
  --bin-dir <dir>      Binary directory. Default: first PATH-visible of ~/.opencode/bin or ~/.local/bin, else <prefix>/bin.
  --languages <list>   Comma-separated project languages, e.g. python,go. A tool bound to a
                       language counts as required only when that language is listed;
                       without this flag, only language-independent tools are required.
  --venv-root <dir>    Root for the dedicated pip tool venvs, one per tool.
  --yes                Do not ask for confirmation after printing the install plan.
  --dry-run            Print what would happen, but do not install.
  -h, --help           Show this help.

Examples:
  scripts/install-security-tools.sh --preflight
  scripts/install-security-tools.sh --languages python,go --preflight
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
  local name languages required installable install_method install_ref asset_pattern role docker_image version_args

  [[ -f "$TOOL_MATRIX_FILE" ]] || die "Security-Tool-Matrix fehlt: $TOOL_MATRIX_FILE"

  # Die Leseliste muss alle Spalten nennen: read schiebt sonst jede weitere Spalte
  # stillschweigend in die letzte Variable.
  while IFS=$'\t' read -r name languages required installable install_method install_ref asset_pattern role docker_image version_args; do
    [[ -z "${name:-}" ]] && continue
    [[ "$name" == \#* ]] && continue
    [[ "$name" == "name" ]] && continue

    case "$required" in
      true|false) ;;
      *) die "Ungültiger required-Wert für $name in $TOOL_MATRIX_FILE: $required" ;;
    esac
    case "$installable" in
      true|false) ;;
      *) die "Ungültiger installable-Wert für $name in $TOOL_MATRIX_FILE: $installable" ;;
    esac
    case "$install_method" in
      github|go|pipx|none) ;;
      *) die "Ungültiger install_method-Wert für $name in $TOOL_MATRIX_FILE: $install_method" ;;
    esac
    [[ -n "${languages:-}" ]] || die "Leeres languages-Feld für $name in $TOOL_MATRIX_FILE. Nutze * für sprachunabhängig."
    if [[ "$install_method" == "github" ]]; then
      [[ -n "${asset_pattern:-}" && "$asset_pattern" != "-" ]] || die "install_method github ohne asset_pattern für $name in $TOOL_MATRIX_FILE."
    fi

    STATUS_TOOLS+=("$name")
    TOOL_REQUIRED["$name"]="$required"
    TOOL_INSTALLABLE["$name"]="$installable"
    TOOL_ROLE["$name"]="$role"
    TOOL_DOCKER_IMAGE["$name"]="$docker_image"
    TOOL_VERSION_ARGS["$name"]="$version_args"
    TOOL_LANGUAGES["$name"]="$languages"
    TOOL_INSTALL_METHOD["$name"]="$install_method"
    TOOL_INSTALL_REF["$name"]="$install_ref"
    TOOL_ASSET_PATTERN["$name"]="$asset_pattern"

    if [[ "$required" == "true" ]]; then
      REQUIRED_TOOLS+=("$name")
    else
      OPTIONAL_TOOLS+=("$name")
    fi
  done < "$TOOL_MATRIX_FILE"

  [[ "${#REQUIRED_TOOLS[@]}" -gt 0 ]] || die "Security-Tool-Matrix enthält keine Pflicht-Tools: $TOOL_MATRIX_FILE"
}

ensure_no_active_project_venv() {
  if [[ -n "${VIRTUAL_ENV:-}" ]]; then
    die "Ein Python-venv ist aktiv ($VIRTUAL_ENV). Der Status darf dieses venv messen; Installationen bleiben aber geschützt. Führe vor --install bitte 'deactivate' aus. Wer Python-Tools in venvs kapseln will, kann danach --method venv nutzen; das verwendet dedizierte k-playbook-Tool-venvs."
  fi
}

ensure_no_project_venv_in_path() {
  local entry
  IFS=':' read -r -a path_entries <<< "$PATH"
  for entry in "${path_entries[@]}"; do
    [[ -z "$entry" ]] && continue
    if is_project_venv_path "$entry"; then
      die "PATH enthält ein typisches Projekt-venv ($entry). Der Status darf dieses venv messen; Installationen bleiben aber geschützt. Führe vor --install bitte 'deactivate' aus bzw. bereinige PATH."
    fi
  done
}

active_project_venv_path() {
  local entry
  if [[ -n "${VIRTUAL_ENV:-}" ]]; then
    printf '%s' "$VIRTUAL_ENV"
    return 0
  fi
  IFS=':' read -r -a path_entries <<< "$PATH"
  for entry in "${path_entries[@]}"; do
    [[ -z "$entry" ]] && continue
    if is_project_venv_path "$entry"; then
      printf '%s' "$entry"
      return 0
    fi
  done
  return 1
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
  if is_project_venv_path "$VENV_ROOT"; then
    die "Die Wurzel der Tool-venvs darf kein Projekt-venv sein: $VENV_ROOT. Nutze pipx oder ein dediziertes Verzeichnis unter ~/.local/share/k-playbook/."
  fi
}

# tool_venv_dir ist der Ort des dedizierten venv eines pip-Tools.
tool_venv_dir() {
  printf '%s' "$VENV_ROOT/$1-venv"
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

# tool_applies meldet, ob ein Tool für die gewählten Projektsprachen zuständig
# ist. Ohne --languages gilt nur Sprachunabhängiges (*) als zuständig: was an eine
# Sprache gebunden ist, lässt sich ohne diese Angabe nicht verlangen.
tool_applies() {
  local tool languages entry wanted
  local -a tool_languages project_languages
  tool="$1"
  languages="${TOOL_LANGUAGES[$tool]:-*}"

  [[ "$languages" == "*" ]] && return 0
  [[ -z "$LANGUAGES" ]] && return 1

  IFS=',' read -r -a tool_languages <<< "$languages"
  IFS=',' read -r -a project_languages <<< "$LANGUAGES"
  for entry in "${tool_languages[@]}"; do
    for wanted in "${project_languages[@]}"; do
      [[ -n "$entry" && "$entry" == "$wanted" ]] && return 0
    done
  done
  return 1
}

# is_required_tool ist die einzige Stelle, an der die Sprachregel angewendet wird:
# Pflicht ist ein Tool nur, wenn die Matrix es so führt und es für die gewählten
# Sprachen zuständig ist.
is_required_tool() {
  local tool item
  tool="$1"
  tool_applies "$tool" || return 1
  for item in "${REQUIRED_TOOLS[@]}"; do
    [[ "$item" == "$tool" ]] && return 0
  done
  return 1
}

tool_languages() {
  printf '%s' "${TOOL_LANGUAGES[$1]:-*}"
}

install_method() {
  printf '%s' "${TOOL_INSTALL_METHOD[$1]:-none}"
}

install_ref() {
  printf '%s' "${TOOL_INSTALL_REF[$1]:--}"
}

asset_pattern() {
  printf '%s' "${TOOL_ASSET_PATTERN[$1]:--}"
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
    if is_required_tool "$tool" && ! has_cmd "$tool"; then
      count=$((count + 1))
    fi
  done
  printf '%s' "$count"
}

# missing_optional_tools listet die optionalen Tools, die für die gewählten
# Sprachen zuständig sind und fehlen.
#
# Sie zählen nicht als Pflicht, dürfen aber nicht unerwähnt bleiben: sonst
# steht in der Tabelle zweimal "fehlt" und darunter "Alle Pflicht-Tools sind
# installiert" — richtig, aber irreführend.
missing_optional_tools() {
  local tool
  for tool in "${OPTIONAL_TOOLS[@]}"; do
    if tool_applies "$tool" && is_installable_tool "$tool" && ! has_cmd "$tool"; then
      printf '%s\n' "$tool"
    fi
  done
}

# install_hint baut den Befehl so, wie er wieder aufzurufen wäre — mitsamt der
# Sprachen, sonst gälte beim nächsten Lauf eine andere Auswahl.
#
# Der Programmpfad ist ein Parameter, weil er sich unterscheidet: im Terminal
# genügt der Aufruf, wie er getippt wurde; die Oberfläche zeigt einen Befehl zum
# Kopieren, der aus jedem Verzeichnis laufen muss.
install_hint() {
  local program="$1" extra="${2:-}"
  if [[ -n "$LANGUAGES" ]]; then
    printf 'bash "%s" --languages %s --install missing%s' "$program" "$LANGUAGES" "$extra"
  else
    printf 'bash "%s" --install missing%s' "$program" "$extra"
  fi
}

# script_path ist der absolute Ort dieses Skripts, unabhaengig davon, wie es
# aufgerufen wurde.
script_path() {
  printf '%s/%s' "$SCRIPT_DIR" "$(basename "${BASH_SOURCE[0]}")"
}

print_preflight() {
  local tool kind status version path image missing_required scope_path
  missing_required="$(missing_required_count)"

  printf 'Security-Tools - Preflight\n'
  printf '%s\n' '--------------------------'
  printf 'Installation: %s\n' "$PLAYBOOK_DIR"
  printf 'Toolliste:  %s\n' "$TOOL_MATRIX_FILE"
  printf 'Bin dir:    %s\n' "$BIN_DIR"
  printf 'Tool-venvs: %s (dediziert, nie ein Projekt-venv)\n' "$VENV_ROOT"
  if scope_path="$(active_project_venv_path)"; then
    printf 'Messkontext: aktives Projekt-venv (%s)\n' "$scope_path"
  else
    printf 'Messkontext: Host-/User-PATH\n'
  fi
  if [[ -n "$LANGUAGES" ]]; then
    printf 'Sprachen:   %s\n' "$LANGUAGES"
  else
    printf 'Sprachen:   nicht angegeben - sprachgebundene Tools gelten als optional\n'
  fi
  printf '\n'
  printf '%-14s %-10s %-9s %-8s %-32s %s\n' 'Tool' 'Sprachen' 'Pflicht' 'Status' 'Version/Pfad' 'Docker-Fallback'
  printf '%-14s %-10s %-9s %-8s %-32s %s\n' '----' '--------' '-------' '------' '------------' '---------------'

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
      printf '%-14s %-10s %-9s %-8s %-32s %s\n' "$tool" "$(tool_languages "$tool")" "$kind" "$status" "${version:0:31}" "$image"
      printf '%-14s %-10s %-9s %-8s %-32s %s\n' '' '' '' '' "$path" ''
    else
      status="fehlt"
      printf '%-14s %-10s %-9s %-8s %-32s %s\n' "$tool" "$(tool_languages "$tool")" "$kind" "$status" "$(tool_role "$tool")" "$image"
    fi
  done < <(all_tools_for_status)

  local -a optional_missing
  mapfile -t optional_missing < <(missing_optional_tools)

  printf '\n'
  if [[ "$missing_required" -eq 0 ]]; then
    printf 'Status: Alle Pflicht-Tools sind installiert.\n'
  else
    printf 'Status: %s Pflicht-Tool(s) fehlen.\n' "$missing_required"
  fi
  if [[ "${#optional_missing[@]}" -gt 0 ]]; then
    printf 'Optional und nicht installiert: %s\n' "$(join_by ', ' "${optional_missing[@]}")"
  fi

  if [[ "$missing_required" -eq 0 && "${#optional_missing[@]}" -eq 0 ]]; then
    return
  fi

  printf '\n'
  printf 'Installationswege:\n'
  printf '  Hinweis: Projekt-venvs sind als Messkontext erlaubt, aber nicht als Installationsziel für diese Tools.\n'
  if [[ "$missing_required" -gt 0 ]]; then
    printf '  Nur die Pflicht:    %s --method auto\n' "$(install_hint "$0")"
    printf '  Tool-venvs:         %s --method venv\n' "$(install_hint "$0")"
  fi
  if [[ "${#optional_missing[@]}" -gt 0 ]]; then
    printf '  Mit den optionalen: %s --method auto\n' "$(install_hint "$0" ' --include-optional')"
    printf '  Tool-venvs plus opt: %s --method venv\n' "$(install_hint "$0" ' --include-optional')"
  fi
  printf '  Docker-Fallback:    %s --method docker\n' "$(install_hint "$0")"
}

# json_escape maskiert die Zeichen, die in einem JSON-String nicht roh stehen
# dürfen. Versionsausgaben und Pfade kommen von fremden Programmen, also wird
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
  local tool status version path image missing_required first scope_path tool_scope tool_scope_message
  local -a optional_missing
  missing_required="$(missing_required_count)"
  mapfile -t optional_missing < <(missing_optional_tools)
  tool_scope="host"
  tool_scope_message="Host-/User-PATH"
  if scope_path="$(active_project_venv_path)"; then
    tool_scope="project-venv"
    tool_scope_message="Aktives Projekt-venv: $scope_path"
  fi

  printf '{\n'
  printf '  "playbookDir": "%s",\n' "$(json_escape "$PLAYBOOK_DIR")"
  printf '  "toolMatrix": "%s",\n' "$(json_escape "$TOOL_MATRIX_FILE")"
  printf '  "binDir": "%s",\n' "$(json_escape "$BIN_DIR")"
  printf '  "venvRoot": "%s",\n' "$(json_escape "$VENV_ROOT")"
  printf '  "toolScope": "%s",\n' "$(json_escape "$tool_scope")"
  printf '  "toolScopePath": "%s",\n' "$(json_escape "${scope_path:-}")"
  printf '  "toolScopeMessage": "%s",\n' "$(json_escape "$tool_scope_message")"
  printf '  "languages": "%s",\n' "$(json_escape "$LANGUAGES")"
  printf '  "missingRequired": %s,\n' "$missing_required"
  printf '  "missingOptional": %s,\n' "${#optional_missing[@]}"
  # Absoluter Pfad: der Befehl soll sich kopieren und von überall ausführen
  # lassen, unabhängig vom Arbeitsverzeichnis des Aufrufs. Die Fassungen
  # entstehen hier, damit die Oberfläche keine Befehle zusammensetzen muss.
  printf '  "installCommand": "%s",\n' \
    "$(json_escape "$(install_hint "$(script_path)" ' --method auto')")"
  printf '  "installCommandVenv": "%s",\n' \
    "$(json_escape "$(install_hint "$(script_path)" ' --method venv')")"
  printf '  "installCommandOptional": "%s",\n' \
    "$(json_escape "$(install_hint "$(script_path)" ' --include-optional --method auto')")"
  printf '  "installCommandOptionalVenv": "%s",\n' \
    "$(json_escape "$(install_hint "$(script_path)" ' --include-optional --method venv')")"
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
    printf '    {"name": "%s", "languages": "%s", "required": %s, "installMethod": "%s", "status": "%s", "version": "%s", "path": "%s", "role": "%s", "dockerImage": "%s"}' \
      "$(json_escape "$tool")" \
      "$(json_escape "$(tool_languages "$tool")")" \
      "$(is_required_tool "$tool" && printf 'true' || printf 'false')" \
      "$(json_escape "$(install_method "$tool")")" \
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

# install_github_binary ist die Matrix-Sicht auf den Release-Weg: die Referenzen
# kommen aus der TSV, der Weg selbst aus der gemeinsamen Bibliothek. In dieser
# Matrix ist der Programmname zugleich die Installationsreferenz, deshalb steht
# er an beiden Stellen; die Basis-Matrix trennt beides.
install_github_binary() {
  local tool
  tool="$1"
  install_release_binary "$tool" "$tool" "$(install_ref "$tool")" "$(asset_pattern "$tool")" "$BIN_DIR"
}

# install_pipx_tool installiert ein pip-Paket host-lokal: bevorzugt mit pipx, sonst
# in ein dediziertes Tool-venv. Der Paketname kommt aus der Matrix und kann Extras
# tragen, etwa paket[extra]; die Binary heißt trotzdem wie die Tool-Spalte.
install_pipx_tool() {
  local tool method package venv_dir
  tool="$1"
  method="$2"
  package="$(install_ref "$tool")"
  venv_dir="$(tool_venv_dir "$tool")"
  [[ -n "$package" && "$package" != "-" ]] || die "Kein pip-Paket in der Matrix für $tool. Spalte install_ref prüfen."
  ensure_bin_dir

  if [[ "$method" == "auto" || "$method" == "pipx" ]]; then
    if has_cmd pipx; then
      log "Installing $tool with pipx: $package"
      run_or_print pipx install --force "$package"
      return
    fi
    if [[ "$method" == "pipx" ]]; then
      die "pipx is not installed. Use --method venv for the dedicated k-playbook tool venv, or install pipx first."
    fi
  fi

  has_cmd python3 || die "python3 is required for the $tool tool-venv install."
  log "Installing $tool into dedicated tool venv: $venv_dir"

  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf 'DRY-RUN: python3 -m venv %q\n' "$venv_dir" >&2
    printf 'DRY-RUN: %q -m pip install --upgrade pip %q\n' "$venv_dir/bin/python" "$package" >&2
    printf 'DRY-RUN: ln -sfn %q %q\n' "$venv_dir/bin/$tool" "$BIN_DIR/$tool" >&2
    return
  fi

  python3 -m venv "$venv_dir"
  "$venv_dir/bin/python" -m pip install --upgrade pip "$package"
  ln -sfn "$venv_dir/bin/$tool" "$BIN_DIR/$tool"
}

# install_go_binary installiert ein Go-Werkzeug mit `go install`. GOBIN zeigt auf
# dasselbe Bin-Verzeichnis wie die übrigen Methoden, damit alle Tools an einem Ort
# liegen und der PATH-Hinweis für alle gilt.
install_go_binary() {
  local tool module
  tool="$1"
  module="$(install_ref "$tool")"
  [[ -n "$module" && "$module" != "-" ]] || die "Kein Go-Modulpfad in der Matrix für $tool. Spalte install_ref prüfen."
  has_cmd go || die "Go ist nicht installiert, wird für $tool aber gebraucht. Installiere Go oder nutze --method docker, falls die Matrix ein Image nennt."
  ensure_bin_dir

  log "Installing $tool with go install into $BIN_DIR: $module"
  GOBIN="$BIN_DIR" run_or_print go install "$module"
}

install_docker_image() {
  local tool image
  tool="$1"
  image="$(docker_image "$tool")"

  if [[ "$image" == "-" ]]; then
    log "Docker-Fallback für $tool ist nicht definiert; nutze pipx/venv/native."
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

  # Verzweigt wird über die Matrix-Spalte install_method, nicht über den
  # Tool-Namen: ein neues Tool ist damit eine Zeile in der TSV und keine Änderung
  # an diesem Skript.
  case "$(install_method "$tool")" in
    pipx)
      case "$method" in
        auto|native|pipx|venv) install_pipx_tool "$tool" "$method" ;;
        docker)
          if ! install_docker_image "$tool"; then
            log "Installiere $tool stattdessen in einem dedizierten Tool-venv."
            install_pipx_tool "$tool" venv
          fi
          ;;
        *) die "Unknown install method for $tool: $method" ;;
      esac
      ;;
    go)
      case "$method" in
        auto|native) install_go_binary "$tool" ;;
        venv) install_go_binary "$tool" ;;
        docker) install_docker_image "$tool" ;;
        pipx) die "$method gilt nur für pip-Tools, $tool wird mit go install geholt." ;;
        *) die "Unknown install method: $method" ;;
      esac
      ;;
    github)
      case "$method" in
        auto|native)
          ensure_bin_dir
          install_github_binary "$tool"
          ;;
        venv)
          ensure_bin_dir
          install_github_binary "$tool"
          ;;
        docker) install_docker_image "$tool" ;;
        pipx) die "$method gilt nur für pip-Tools, $tool kommt aus einem GitHub-Release." ;;
        *) die "Unknown install method: $method" ;;
      esac
      ;;
    none)
      die "$tool ist laut Matrix nicht installierbar."
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
      # Folgt derselben Sprachregel wie die Statuszeile: was der Preflight nicht als
      # fehlende Pflicht zählt, wird hier auch nicht installiert. Die expliziten
      # Ziele required und all bleiben davon unberührt.
      for tool in "${REQUIRED_TOOLS[@]}"; do
        if is_required_tool "$tool" && is_installable_tool "$tool" && ! has_cmd "$tool"; then
          printf '%s\n' "$tool"
        fi
      done
      if [[ "$INCLUDE_OPTIONAL" -eq 1 ]]; then
        # Auch die optionalen folgen der Sprachregel: ein Go-Projekt soll sich
        # kein Python-Werkzeug einhandeln, nur weil es die optionalen mitnimmt.
        missing_optional_tools
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
      --languages)
        LANGUAGES="${2:-}"
        [[ -n "$LANGUAGES" ]] || die "--languages requires a comma-separated list."
        shift 2
        ;;
      --venv-root)
        VENV_ROOT="${2:-}"
        [[ -n "$VENV_ROOT" ]] || die "--venv-root requires a path."
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

  ensure_host_tool_scope
  # Ein Lauf, der nichts schreibt, wird nicht abgewiesen: --dry-run soll gerade
  # in der Konstellation laufen, die schreibend abbräche.
  if [[ "$DRY_RUN" -ne 1 ]]; then
    ensure_target_owner "$BIN_DIR" \
      "bash \"$(script_path)\" --install ${INSTALL_SPEC:-missing}" \
      "sudo bash \"$(script_path)\" --install ${INSTALL_SPEC:-missing} --bin-dir /usr/local/bin"
  fi

  validate_install_spec

  mapfile -t tools < <(selected_tools)
  count="${#tools[@]}"

  if [[ "$count" -eq 0 ]]; then
    printf '\nNichts zu installieren für Auswahl: %s\n' "$INSTALL_SPEC"
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
