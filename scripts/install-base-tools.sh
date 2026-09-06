#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PLAYBOOK_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
# Der Release-Weg und der Guard auf das Installationsziel liegen in einer
# gemeinsamen Bibliothek, die auch install-security-tools.sh sourct. Gesucht
# wird relativ zum Ort dieses Skripts, nicht zum Arbeitsverzeichnis: die
# Skripte laufen aus der Installation heraus und aus beliebigen Verzeichnissen.
# shellcheck source=lib/install-common.sh
source "$SCRIPT_DIR/lib/install-common.sh"
# Die Matrix liegt neben diesem Skript, damit beide zusammen verschoben werden
# können.
TOOL_MATRIX_FILE="${K_BASE_TOOLS_MATRIX:-$SCRIPT_DIR/base-tools.tsv}"

# Rückgabewert für „für dieses Werkzeug gibt es hier keinen Weg". Er trennt die
# Fälle 3 und 4 der Rangfolge sowie die Methode none vom echten Fehlschlag: dort
# ist der ausgegebene Befehl das Ergebnis, nicht ein Zwischenschritt vor einem
# stillen Umfallen. 1 bleibt dem Abbruch über die() vorbehalten.
EXIT_NO_LOCAL_WAY=3

TOOLS=()
declare -A TOOL_GROUP=()
declare -A TOOL_ROLE=()
declare -A TOOL_GUARDED=()
declare -A TOOL_METHODS=()
declare -A TOOL_APT_PACKAGE=()
declare -A TOOL_GITHUB_REPO=()
declare -A TOOL_ASSET_REF=()
declare -A TOOL_ASSET_PATTERN=()

INSTALL=0
JSON_OUTPUT=0
YES=0
DRY_RUN=0
# Eigene Optionen und ein eigener Namensraum: die K_SECURITY_TOOLS_*-Variablen
# gehören dem anderen Skript. Ein fest verdrahtetes ~/.local/bin ließe zudem den
# gemeinsamen Guard ins Leere zeigen, weil es dann gar kein auflösbares Ziel
# gäbe.
PREFIX="${K_BASE_TOOLS_PREFIX:-$HOME/.local}"

# default_bin_dir wählt das erste PATH-sichtbare Ziel. Ein installiertes
# Werkzeug, das nicht im PATH steht, meldete der Kontextbefund weiterhin als
# fehlend — der Erfolg widerspräche sich mit dem Befund.
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

BIN_DIR="${K_BASE_TOOLS_BIN_DIR:-$(default_bin_dir)}"

usage() {
  cat <<USAGE
Usage: install-base-tools.sh [--preflight]
       install-base-tools.sh --install [--yes] [--dry-run]

Preflight und Installer für die Basis-Werkzeuge, die k-playbook selbst aufruft.
Es werden keine Projektdateien geschrieben.

Werkzeugmatrix:
  $TOOL_MATRIX_FILE

Optionen:
  --preflight       Nur den Zustand zeigen. Das ist die Voreinstellung.
  --json            Den Zustand als JSON ausgeben und beenden. Rein lesend.
  --install         Die fehlenden Werkzeuge installieren.
  --prefix <dir>    User-lokales Prefix für den Release-Weg. Standard: ~/.local.
  --bin-dir <dir>   Bin-Verzeichnis. Standard: erstes PATH-sichtbares aus
                    ~/.opencode/bin oder ~/.local/bin, sonst <prefix>/bin.
  --yes             Keine Rückfrage stellen. Für eine RUN-Zeile im Dockerfile.
  --dry-run         Zeigen, was geschähe, aber nichts installieren.
  -h, --help        Diese Hilfe zeigen.

Umgebung:
  K_BASE_TOOLS_MATRIX, K_BASE_TOOLS_PREFIX, K_BASE_TOOLS_BIN_DIR

Rangfolge der Installationswege, je Eintrag:
  1. root und apt-get vorhanden        -> apt-get install
  2. sonst, wenn die Matrix github führt -> user-lokaler Release-Weg
  3. sonst, apt-only mit apt-get       -> sudo apt-get ... ausgeben (Exit $EXIT_NO_LOCAL_WAY)
  4. sonst, apt-only ohne apt-get      -> Grund benennen (Exit $EXIT_NO_LOCAL_WAY)

k-playbook eskaliert nie selbst zu root: der sudo-Befehl wird gezeigt, nie
ausgeführt, und dieses Skript startet sich nicht per sudo neu.
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

load_tool_matrix() {
  local name group role guarded methods apt_package github_repo asset_ref asset_pattern method

  [[ -f "$TOOL_MATRIX_FILE" ]] || die "Matrix der Basis-Werkzeuge fehlt: $TOOL_MATRIX_FILE"

  # Die Leseliste muss alle Spalten nennen: read schiebt sonst jede weitere
  # Spalte stillschweigend in die letzte Variable.
  while IFS=$'\t' read -r name group role guarded methods apt_package github_repo asset_ref asset_pattern; do
    [[ -z "${name:-}" ]] && continue
    [[ "$name" == \#* ]] && continue
    [[ "$name" == "name" ]] && continue

    [[ -n "${methods:-}" ]] || die "Leeres methods-Feld für $name in $TOOL_MATRIX_FILE."
    for method in ${methods//,/ }; do
      case "$method" in
        apt|github|none) ;;
        *) die "Ungültiger methods-Wert für $name in $TOOL_MATRIX_FILE: $method" ;;
      esac
    done
    if has_method "$methods" apt; then
      [[ -n "${apt_package:-}" && "$apt_package" != "-" ]] || die "Methode apt ohne apt_package für $name in $TOOL_MATRIX_FILE."
    fi
    if has_method "$methods" github; then
      [[ -n "${github_repo:-}" && "$github_repo" != "-" ]] || die "Methode github ohne github_repo für $name in $TOOL_MATRIX_FILE."
      [[ -n "${asset_pattern:-}" && "$asset_pattern" != "-" ]] || die "Methode github ohne asset_pattern für $name in $TOOL_MATRIX_FILE."
    fi

    TOOLS+=("$name")
    TOOL_GROUP["$name"]="$group"
    TOOL_ROLE["$name"]="$role"
    TOOL_GUARDED["$name"]="$guarded"
    TOOL_METHODS["$name"]="$methods"
    TOOL_APT_PACKAGE["$name"]="$apt_package"
    TOOL_GITHUB_REPO["$name"]="$github_repo"
    # Ohne eigene Angabe bindet {tool} an den Programmnamen. Für rg steht in der
    # Matrix ripgrep: im Asset-Muster taugt der Programmname nicht.
    if [[ -z "${asset_ref:-}" || "$asset_ref" == "-" ]]; then
      TOOL_ASSET_REF["$name"]="$name"
    else
      TOOL_ASSET_REF["$name"]="$asset_ref"
    fi
    TOOL_ASSET_PATTERN["$name"]="$asset_pattern"
  done < "$TOOL_MATRIX_FILE"

  [[ "${#TOOLS[@]}" -gt 0 ]] || die "Matrix der Basis-Werkzeuge enthält keine Einträge: $TOOL_MATRIX_FILE"
}

# has_method meldet, ob eine Methodenliste eine bestimmte Methode führt.
has_method() {
  local entry
  for entry in ${1//,/ }; do
    [[ "$entry" == "$2" ]] && return 0
  done
  return 1
}

# ---------------------------------------------------------------------------
# Anwesenheit
# ---------------------------------------------------------------------------

# tool_present prüft die Anwesenheit im PATH — hier über `command -v`, weil
# dieses Skript selbst bash ist; die Go-Seite nutzt dafür exec.LookPath.
#
# Führt der Eintrag eine Gruppe, gilt er als vorhanden, sobald ein Mitglied der
# Gruppe da ist. Ohne das meldete der Befund auf einem Host mit curl dauerhaft
# wget als fehlend — ein vermeidbarer Fehlalarm.
tool_present() {
  local tool group member
  tool="$1"
  group="${TOOL_GROUP[$tool]:--}"

  if [[ "$group" == "-" || -z "$group" ]]; then
    has_cmd "$tool" && return 0
    return 1
  fi

  for member in "${TOOLS[@]}"; do
    [[ "${TOOL_GROUP[$member]:--}" == "$group" ]] || continue
    has_cmd "$member" && return 0
  done
  return 1
}

# group_members listet die Mitglieder der Gruppe eines Eintrags, ihn selbst
# eingeschlossen.
group_members() {
  local tool group member
  tool="$1"
  group="${TOOL_GROUP[$tool]:--}"
  if [[ "$group" == "-" || -z "$group" ]]; then
    printf '%s' "$tool"
    return
  fi
  local -a members=()
  for member in "${TOOLS[@]}"; do
    [[ "${TOOL_GROUP[$member]:--}" == "$group" ]] && members+=("$member")
  done
  join_by ' oder ' "${members[@]}"
}

# missing_tools listet, was fehlt. Von einer Gruppe erscheint nur das erste
# Mitglied: installiert wird eines, nicht alle.
missing_tools() {
  local tool group
  local -a seen_groups=()
  for tool in "${TOOLS[@]}"; do
    tool_present "$tool" && continue
    group="${TOOL_GROUP[$tool]:--}"
    if [[ "$group" != "-" && -n "$group" ]]; then
      if [[ " ${seen_groups[*]:-} " == *" $group "* ]]; then
        continue
      fi
      seen_groups+=("$group")
    fi
    printf '%s\n' "$tool"
  done
}

# ---------------------------------------------------------------------------
# Schutz des Projekt-venv (rules/tool-install-scope.md)
# ---------------------------------------------------------------------------

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

active_project_venv_path() {
  local entry
  local -a path_entries
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

# ensure_host_tool_scope gilt nur für schreibende Läufe. --preflight und --json
# dürfen ein aktives venv messen, kennzeichnen es aber als Messkontext.
ensure_host_tool_scope() {
  local entry
  local -a path_entries
  if [[ -n "${VIRTUAL_ENV:-}" ]]; then
    die "Ein Python-venv ist aktiv ($VIRTUAL_ENV). Der Status darf dieses venv messen; Installationen bleiben aber geschützt. Führe vor --install bitte 'deactivate' aus."
  fi
  IFS=':' read -r -a path_entries <<< "$PATH"
  for entry in "${path_entries[@]}"; do
    [[ -z "$entry" ]] && continue
    if is_project_venv_path "$entry"; then
      die "PATH enthält ein typisches Projekt-venv ($entry). Der Status darf dieses venv messen; Installationen bleiben aber geschützt. Führe vor --install bitte 'deactivate' aus bzw. bereinige PATH."
    fi
  done
  if is_project_venv_path "$BIN_DIR"; then
    die "Installationsziel liegt in einem typischen Projekt-venv: $BIN_DIR. Nutze ein user-lokales Bin-Verzeichnis wie ~/.opencode/bin oder ~/.local/bin."
  fi
}

# ---------------------------------------------------------------------------
# Ausgabe
# ---------------------------------------------------------------------------

script_path() {
  printf '%s/%s' "$SCRIPT_DIR" "$(basename "${BASH_SOURCE[0]}")"
}

install_hint() {
  printf 'bash "%s" --install' "$1"
}

apt_command() {
  printf 'sudo apt-get install -y %s' "${TOOL_APT_PACKAGE[$1]}"
}

is_root() {
  [[ "$(id -u)" -eq 0 ]]
}

print_preflight() {
  local tool status scope_path
  local -a missing
  mapfile -t missing < <(missing_tools)

  printf 'Basis-Werkzeuge - Preflight\n'
  printf '%s\n' '---------------------------'
  printf 'Installation: %s\n' "$PLAYBOOK_DIR"
  printf 'Werkzeugliste: %s\n' "$TOOL_MATRIX_FILE"
  printf 'Bin dir:       %s\n' "$BIN_DIR"
  if scope_path="$(active_project_venv_path)"; then
    printf 'Messkontext:   aktives Projekt-venv (%s)\n' "$scope_path"
  else
    printf 'Messkontext:   Host-/User-PATH\n'
  fi
  printf '\n'
  printf '%-10s %-8s %-12s %s\n' 'Werkzeug' 'Status' 'Wege' 'Rolle bzw. Pfad'
  printf '%-10s %-8s %-12s %s\n' '--------' '------' '----' '---------------'

  for tool in "${TOOLS[@]}"; do
    if has_cmd "$tool"; then
      status="ok"
      printf '%-10s %-8s %-12s %s\n' "$tool" "$status" "${TOOL_METHODS[$tool]}" "$(command -v "$tool")"
    elif tool_present "$tool"; then
      status="Gruppe"
      printf '%-10s %-8s %-12s %s\n' "$tool" "$status" "${TOOL_METHODS[$tool]}" "genügt: $(group_members "$tool")"
    else
      status="fehlt"
      printf '%-10s %-8s %-12s %s\n' "$tool" "$status" "${TOOL_METHODS[$tool]}" "${TOOL_ROLE[$tool]}"
    fi
  done

  printf '\n'
  if [[ "${#missing[@]}" -eq 0 ]]; then
    printf 'Status: Alle Basis-Werkzeuge sind vorhanden.\n'
    return
  fi

  printf 'Status: %s Basis-Werkzeug(e) fehlen: %s\n' "${#missing[@]}" "$(join_by ', ' "${missing[@]}")"
  printf '\n'
  printf 'Installation:\n'
  printf '  %s\n' "$(install_hint "$(script_path)")"
  printf '\n'
  printf 'Ein fehlendes Basis-Werkzeug warnt und blockiert nicht. Was ohne das\n'
  printf 'jeweilige Werkzeug ausfällt, steht oben in der Spalte Rolle.\n'
}

json_escape() {
  local text="$1"
  text="${text//\\/\\\\}"
  text="${text//\"/\\\"}"
  text="${text//$'\t'/\\t}"
  text="${text//$'\r'/}"
  text="${text//$'\n'/\\n}"
  printf '%s' "$text"
}

print_preflight_json() {
  local tool status path first scope_path tool_scope tool_scope_message
  local -a missing
  mapfile -t missing < <(missing_tools)
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
  printf '  "toolScope": "%s",\n' "$(json_escape "$tool_scope")"
  printf '  "toolScopePath": "%s",\n' "$(json_escape "${scope_path:-}")"
  printf '  "toolScopeMessage": "%s",\n' "$(json_escape "$tool_scope_message")"
  printf '  "missing": %s,\n' "${#missing[@]}"
  printf '  "installCommand": "%s",\n' "$(json_escape "$(install_hint "$(script_path)")")"
  printf '  "tools": [\n'

  first=1
  for tool in "${TOOLS[@]}"; do
    if has_cmd "$tool"; then
      status="ok"
      path="$(command -v "$tool")"
    elif tool_present "$tool"; then
      status="group-ok"
      path=""
    else
      status="missing"
      path=""
    fi

    [[ "$first" -eq 0 ]] && printf ',\n'
    first=0
    printf '    {"name": "%s", "group": "%s", "methods": "%s", "guarded": "%s", "status": "%s", "path": "%s", "role": "%s"}' \
      "$(json_escape "$tool")" \
      "$(json_escape "${TOOL_GROUP[$tool]}")" \
      "$(json_escape "${TOOL_METHODS[$tool]}")" \
      "$(json_escape "${TOOL_GUARDED[$tool]}")" \
      "$status" \
      "$(json_escape "$path")" \
      "$(json_escape "${TOOL_ROLE[$tool]}")"
  done

  printf '\n  ]\n'
  printf '}\n'
}

# ---------------------------------------------------------------------------
# Installation
# ---------------------------------------------------------------------------

ensure_bin_dir() {
  if [[ "$DRY_RUN" -eq 0 ]]; then
    mkdir -p "$BIN_DIR"
  fi

  case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *)
      log "WARN: $BIN_DIR ist nicht in PATH. Installierte Werkzeuge sind erst nach einer PATH-Anpassung aufrufbar — und der Kontextbefund meldete sie bis dahin weiterhin als fehlend."
      log "      export PATH=\"$BIN_DIR:\$PATH\""
      ;;
  esac
}

install_apt_package() {
  local tool package
  tool="$1"
  package="${TOOL_APT_PACKAGE[$tool]}"
  log "Installiere $tool über apt: $package"
  DEBIAN_FRONTEND=noninteractive run_or_print apt-get update
  DEBIAN_FRONTEND=noninteractive run_or_print apt-get install -y --no-install-recommends "$package"
}

# install_entry geht die Rangfolge für genau einen Eintrag durch. Jeder Fall
# endet benannt; keiner läuft ins Leere. Der Rückgabewert unterscheidet Erfolg
# (0) von „hier gibt es keinen Weg" ($EXIT_NO_LOCAL_WAY).
install_entry() {
  local tool methods
  tool="$1"
  methods="${TOOL_METHODS[$tool]}"

  # Fall 1: root und apt-get vorhanden. Das Ziel ist systemweit und hängt nicht
  # an $HOME, deshalb gilt der Eigentümer-Guard hier nicht.
  if has_method "$methods" apt && is_root && has_cmd apt-get; then
    install_apt_package "$tool"
    return 0
  fi

  # Fall 2: user-lokaler Release-Weg — auch dann, wenn apt-get vorhanden ist.
  # Das ist der häufigste Entwicklerfall: Ubuntu-Host, apt da, kein root.
  if has_method "$methods" github; then
    # Der Guard steht hier und nicht am Skriptanfang: erst je Eintrag und erst
    # nach `command -v apt-get` steht fest, ob überhaupt user-lokal geschrieben
    # wird. Ein Guard am Anfang bräche `sudo ./install-base-tools.sh --install`
    # auf Ubuntu zu Unrecht ab, wo jeder apt-Eintrag in Fall 1 fällt.
    #
    # Er steht außerdem vor ensure_bin_dir: ein mkdir auf ein fremdes Ziel
    # schlüge sonst zuerst fehl und meldete etwas anderes als das eigentliche
    # Problem.
    if [[ "$DRY_RUN" -ne 1 ]]; then
      ensure_target_owner "$BIN_DIR" \
        "bash \"$(script_path)\" --install" \
        "sudo bash \"$(script_path)\" --install --bin-dir /usr/local/bin"
    fi
    if has_method "$methods" apt && has_cmd apt-get; then
      printf 'Hinweis: systemweit ginge %s auch über apt:\n' "$tool"
      printf '  %s\n' "$(apt_command "$tool")"
      printf 'Installiert wird trotzdem user-lokal nach %s — das braucht keinen root.\n' "$BIN_DIR"
    fi
    # ensure_bin_dir steht vor dem Trockenlauf-Zweig, damit auch dort der
    # PATH-Hinweis erscheint: ein Ziel außerhalb des PATH meldete der
    # Kontextbefund nach der Installation weiterhin als fehlend.
    ensure_bin_dir
    # Ein Trockenlauf geht nicht ins Netz. Die Auflösung des Assets fragt die
    # GitHub-API; das ist genau das, was ein --dry-run im Image-Build und im
    # Test nicht tun soll. Gezeigt wird deshalb das Ziel, nicht die aufgelöste
    # URL.
    if [[ "$DRY_RUN" -eq 1 ]]; then
      printf 'DRY-RUN: %s user-lokal aus dem Release von %s nach %s/%s installieren\n' \
        "$tool" "${TOOL_GITHUB_REPO[$tool]}" "$BIN_DIR" "$tool"
      printf 'DRY-RUN: Asset-Muster %s (nicht aufgelöst, ein Trockenlauf geht nicht ins Netz)\n' \
        "${TOOL_ASSET_PATTERN[$tool]}"
      return 0
    fi
    install_release_binary \
      "$tool" \
      "${TOOL_ASSET_REF[$tool]}" \
      "${TOOL_GITHUB_REPO[$tool]}" \
      "${TOOL_ASSET_PATTERN[$tool]}" \
      "$BIN_DIR"
    return 0
  fi

  # Fall 3: apt-only auf einem Host mit apt-get, aber ohne root. Der Befehl ist
  # hier das Ergebnis, nicht ein Hinweis vor einem weiteren Versuch.
  if has_method "$methods" apt && has_cmd apt-get; then
    printf '%s: kein user-lokaler Weg. Für dieses Werkzeug gibt es keinen sinnvollen\n' "$tool"
    printf '  GitHub-Release, den man nach %s entpacken könnte. Systemweit:\n' "$BIN_DIR"
    printf '  %s\n' "$(apt_command "$tool")"
    return "$EXIT_NO_LOCAL_WAY"
  fi

  # Fall 4: apt-only auf einem Host ohne apt-get — Alpine, RHEL, macOS, gleich
  # ob root oder nicht. Ohne diesen Fall bliebe etwa git als root im
  # Alpine-Container ohne Ergebnis.
  if has_method "$methods" apt; then
    printf '%s: kein Weg auf diesem Host. Der Eintrag ist apt-only, aber es gibt kein\n' "$tool"
    printf '  apt auf diesem System. Installiere %s über den Paketmanager der\n' "$tool"
    printf '  Distribution (apk, dnf, brew, ...).\n'
    return "$EXIT_NO_LOCAL_WAY"
  fi

  # Methode none: niemand installiert das hier.
  printf '%s: kein Installationsweg vorgesehen (Methode none in der Matrix).\n' "$tool"
  return "$EXIT_NO_LOCAL_WAY"
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
        INSTALL=1
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
}

main() {
  local tool answer status no_way
  local -a missing
  load_tool_matrix
  parse_args "$@"

  # --json ist rein lesend: es beschreibt den Zustand und installiert nichts.
  if [[ "$JSON_OUTPUT" -eq 1 ]]; then
    print_preflight_json
    exit 0
  fi

  print_preflight

  if [[ "$INSTALL" -ne 1 ]]; then
    exit 0
  fi

  # Der venv-Schutz gilt für jeden schreibenden Lauf, auch für --dry-run: er
  # betrifft die Auswahl des Ziels und nicht den Schreibvorgang.
  ensure_host_tool_scope

  mapfile -t missing < <(missing_tools)
  if [[ "${#missing[@]}" -eq 0 ]]; then
    printf '\nNichts zu installieren.\n'
    exit 0
  fi

  printf '\nInstallationsplan\n'
  printf '%s\n' '-----------------'
  printf 'Bin dir:  %s\n' "$BIN_DIR"
  if is_root; then
    printf 'Rechte:   root (effektive UID 0)\n'
  else
    printf 'Rechte:   normaler Benutzer\n'
  fi
  if has_cmd apt-get; then
    printf 'apt-get:  vorhanden\n'
  else
    printf 'apt-get:  nicht vorhanden\n'
  fi
  printf 'Werkzeuge:\n'
  for tool in "${missing[@]}"; do
    printf '  - %s (%s; Wege: %s)\n' "$tool" "${TOOL_ROLE[$tool]}" "${TOOL_METHODS[$tool]}"
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

  printf '\n'
  no_way=0
  for tool in "${missing[@]}"; do
    status=0
    install_entry "$tool" || status=$?
    if [[ "$status" -eq "$EXIT_NO_LOCAL_WAY" ]]; then
      no_way=1
    elif [[ "$status" -ne 0 ]]; then
      exit "$status"
    fi
  done

  if [[ "$no_way" -eq 1 ]]; then
    printf '\nMindestens ein Werkzeug hat auf diesem Host keinen Weg, den dieses Skript\n'
    printf 'gehen darf. Die Befehle oben sind das Ergebnis, kein Fehlschlag.\n'
    exit "$EXIT_NO_LOCAL_WAY"
  fi

  printf '\nFinaler Status\n'
  printf '%s\n' '--------------'
  print_preflight
}

main "$@"
