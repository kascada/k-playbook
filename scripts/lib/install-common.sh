#!/usr/bin/env bash
# Gemeinsame Bausteine der beiden Installationsskripte unter scripts/.
#
# Diese Datei wird gesourct, nicht ausgeführt. Sie liegt neben den Skripten und
# wird mit dem Clone in jedes Zielprojekt ausgeliefert; gefunden wird sie über
# den Ort des aufrufenden Skripts (BASH_SOURCE), nie über das
# Arbeitsverzeichnis — die Skripte werden aus der Installation heraus und aus
# beliebigen Verzeichnissen aufgerufen.
#
# Warum gemeinsam: install-security-tools.sh und install-base-tools.sh gehen
# denselben Release-Weg und führen denselben Guard auf das Installationsziel.
# Doppelter Code hieße, dass die Zusage „die bestehenden Muster bleiben gültig"
# nach der ersten einseitigen Änderung driftet.
#
# Vertrag an das sourcende Skript: Es definiert vor dem ersten Aufruf
#   die() ... , log() ... , run_or_print() ...
# und setzt DRY_RUN auf 0 oder 1. Alles andere bringt diese Datei mit.

# has_cmd meldet, ob ein Programm im PATH liegt. Reine Anwesenheitsprüfung,
# ohne das Programm zu starten.
has_cmd() {
  command -v "$1" >/dev/null 2>&1
}

# ---------------------------------------------------------------------------
# Guard auf das Installationsziel
# ---------------------------------------------------------------------------

# nearest_existing_dir liefert das nächste vorhandene Verzeichnis oberhalb eines
# Pfads — den Pfad selbst, wenn es ihn schon gibt.
#
# Beim ersten Lauf existiert ~/.local/bin oft noch nicht. Eine Eigentümerprüfung
# auf einen nicht vorhandenen Pfad wäre undefiniert, und genau dieser Fall
# trifft den Image-Build.
nearest_existing_dir() {
  local path
  path="$1"
  while [[ -n "$path" && "$path" != "/" && "$path" != "." && ! -d "$path" ]]; do
    path="$(dirname "$path")"
  done
  printf '%s' "$path"
}

# path_owner_uid liest die Eigentümer-UID eines Pfads. GNU coreutils kennt
# stat -c, BSD und macOS stat -f; geprüft wird die UID und nicht der Name,
# damit kein Namensdienst befragt werden muss.
path_owner_uid() {
  stat -c '%u' "$1" 2>/dev/null || stat -f '%u' "$1" 2>/dev/null || printf ''
}

# path_owner_name ist nur für die Meldung da. Fehlt der Name, bleibt die UID
# stehen — die Aussage trägt auch ohne Namensdienst.
path_owner_name() {
  local uid name
  uid="$1"
  name="$(getent passwd "$uid" 2>/dev/null | cut -d: -f1 || true)"
  if [[ -n "$name" ]]; then
    printf '%s (UID %s)' "$name" "$uid"
  else
    printf 'UID %s' "$uid"
  fi
}

# ensure_target_owner ist der gemeinsame Guard beider Installationsskripte. Es
# gibt genau ein Abbruchkriterium: Das aufgelöste Installationsziel — oder,
# wenn es noch nicht existiert, sein nächstes vorhandenes Elternverzeichnis —
# gehört nicht der effektiven UID.
#
# Weder root noch $HOME noch SUDO_USER sind eigenständige Abbruchgründe; sie
# kommen nur im Meldungstext als Erklärung vor. Andernfalls bräche der
# ausdrücklich erlaubte systemweite Aufruf mit --bin-dir /usr/local/bin ebenso
# ab wie `sudo -H` (bei dem gar kein Pfad gebrochen ist) und `sudo -u <user> -H`
# — eine Rechteabgabe, keine Verwechslung. Die Doktrin dahinter steht in
# installer/docs/architecture.md, Abschnitt „Root-Doktrin beider Skripte".
#
# Aufgerufen wird nur, wo geschrieben wird: --preflight, --json und --dry-run
# schreiben nichts, und ein Abbruch nähme dort gerade die Diagnose, mit der man
# den Fall überhaupt versteht.
#
#   ensure_target_owner <ziel> <richtiger-aufruf> <systemweiter-aufruf>
#
# Die beiden Aufrufzeilen kommen vom sourcenden Skript: sie unterscheiden sich
# in Programmname und Argumenten.
ensure_target_owner() {
  local target correct systemwide resolved owner_uid effective_uid
  target="$1"
  correct="${2:-}"
  systemwide="${3:-}"
  resolved="$(nearest_existing_dir "$target")"
  effective_uid="$(id -u)"

  [[ -n "$resolved" ]] || return 0
  owner_uid="$(path_owner_uid "$resolved")"
  [[ -n "$owner_uid" ]] || return 0
  [[ "$owner_uid" == "$effective_uid" ]] && return 0

  die "Installationsziel $target gehört nicht dem ausführenden Benutzer.
  Geprüfter Pfad:  $resolved
  Eigentümer:      $(path_owner_name "$owner_uid")
  Effektive UID:   $effective_uid$(
    if [[ -n "${SUDO_USER:-}" ]]; then printf '\n  Aufruf über sudo von: %s' "$SUDO_USER"; fi)
  Zielpfad und Prefix hängen an \$HOME (hier: ${HOME:-unbekannt}). Unter sudo bleibt
  \$HOME oft das des Aufrufers, die Dateien gehören danach aber root — sie liegen
  dann entweder im falschen Home oder mit falschem Eigentümer im richtigen.
  k-playbook eskaliert nie selbst zu root und startet sich nie per sudo neu.
  Richtig ist der Aufruf ohne sudo:
    $correct
  Ist ein systemweites Ziel gemeint, nenne es ausdrücklich — das ist der Weg,
  den die Doktrin erlaubt:
    $systemwide"
}

# ---------------------------------------------------------------------------
# Release-Weg: Plattform, Asset-Auflösung, Download, Installation
# ---------------------------------------------------------------------------

download_file() {
  local url dest
  url="$1"
  dest="$2"

  if has_cmd curl; then
    run_or_print curl -L --fail --show-error --output "$dest" "$url"
  elif has_cmd wget; then
    run_or_print wget -O "$dest" "$url"
  else
    die "curl oder wget ist für native Downloads erforderlich."
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

# resolve_release_asset löst ein Release-Asset gegen ein Muster auf und gibt
# drei Zeilen aus: Tag, Download-URL, Asset-Name.
#
#   resolve_release_asset <asset_ref> <repo> <muster> <os> <arch> [releases_json]
#
# os und arch werden übergeben statt hier ermittelt: nur so lässt sich die
# Platzhalter-Auflösung für jede Plattform prüfen, ohne auf ihr zu laufen.
# releases_json ist optional — ohne Angabe wird die GitHub-API befragt, mit
# Angabe die Datei gelesen. Das ist der Weg, auf dem die Tests die Auflösung
# ohne Netzzugriff prüfen.
#
# asset_ref ist die Installationsreferenz und nicht der Programmname: das Asset
# von BurntSushi/ripgrep heißt ripgrep-…, das Programm heißt rg.
resolve_release_asset() {
  local asset_ref repo pattern os arch releases_json
  asset_ref="$1"
  repo="$2"
  pattern="$3"
  os="$4"
  arch="$5"
  releases_json="${6:-}"

  python3 - "$repo" "$asset_ref" "$os" "$arch" "$pattern" "$releases_json" <<'PY'
import json
import re
import sys
import urllib.request

repo, tool, os_name, arch, pattern, releases_file = sys.argv[1:7]

# Die Platzhalter decken die Namenskonventionen ab, die unter den Werkzeugen
# tatsächlich vorkommen. Alles andere im Muster ist regulärer Ausdruck und
# steht so in der Matrix.
#
# {arch_raw} und {vendor_os} sind für die Rust-Target-Triples da: ripgrep
# benennt seine Assets nach <arch_raw>-<vendor>-<os>-<libc>, und keine
# Kombination der übrigen Platzhalter erzeugt x86_64-unknown-linux-musl oder
# apple-darwin. Der Libc-Teil ist unter Linux nicht einheitlich — x86_64 kommt
# als musl, aarch64 als gnu —, deshalb steht dort eine Alternative statt eines
# festen Tokens. Die Erweiterung ist rein additiv: kein bestehender Platzhalter
# ändert seine Bedeutung.
placeholders = {
    "{tool}": re.escape(tool),
    "{version}": r".*",
    "{os}": os_name,
    "{arch}": arch,
    "{arch_x64}": "x64" if arch == "amd64" else "arm64",
    "{os_cap}": "Linux" if os_name == "linux" else "macOS",
    "{arch_bits}": "64bit" if arch == "amd64" else "ARM64",
    "{arch_raw}": "x86_64" if arch == "amd64" else "aarch64",
    "{vendor_os}": "unknown-linux-(?:musl|gnu)" if os_name == "linux" else "apple-darwin",
}
for key, value in placeholders.items():
    pattern = pattern.replace(key, value)

if releases_file:
    with open(releases_file, encoding="utf-8") as handle:
        data = json.load(handle)
else:
    url = f"https://api.github.com/repos/{repo}/releases/latest"
    req = urllib.request.Request(
        url,
        headers={"Accept": "application/vnd.github+json", "User-Agent": "k-playbook"},
    )
    with urllib.request.urlopen(req, timeout=30) as response:
        data = json.load(response)

assets = data.get("assets", [])
tag = data.get("tag_name", "latest")

regex = re.compile(pattern)
for asset in assets:
    name = asset.get("name", "")
    # Jedes Asset hat ein .sha256-Geschwister. Ein laxes Muster träfe die
    # Prüfsumme zuerst, und installiert würde dann eine Textdatei.
    if name.endswith(".sha256"):
        continue
    if regex.match(name):
        print(tag)
        print(asset["browser_download_url"])
        print(name)
        raise SystemExit(0)

names = ", ".join(asset.get("name", "") for asset in assets)
raise SystemExit(f"no matching release asset for {tool}; pattern {pattern}; available: {names}")
PY
}

# latest_asset ist resolve_release_asset für die Plattform, auf der wir laufen.
latest_asset() {
  local os arch
  read -r os arch < <(platform_key)
  resolve_release_asset "$1" "$2" "$3" "$os" "$arch"
}

# install_release_binary holt ein Release-Asset und legt die Binary im
# Zielverzeichnis ab.
#
#   install_release_binary <programm> <asset_ref> <repo> <muster> <bin_dir>
#
# programm ist der Name, unter dem die Binary im Archiv liegt und danach im
# PATH steht; asset_ref bindet {tool} im Muster.
install_release_binary() {
  local program asset_ref repo pattern bin_dir
  local tmp_dir asset_info tag url asset archive extract_dir candidate installed
  local -a asset_lines
  program="$1"
  asset_ref="$2"
  repo="$3"
  pattern="$4"
  bin_dir="$5"

  [[ -n "$repo" && "$repo" != "-" ]] || die "Kein GitHub-Repo in der Matrix für $program. Spalte für die Installationsreferenz prüfen."
  [[ -n "$pattern" && "$pattern" != "-" ]] || die "Kein asset_pattern in der Matrix für $program."

  has_cmd python3 || die "python3 is required to resolve latest GitHub releases."
  has_cmd tar || die "tar is required to extract release archives."

  asset_info="$(latest_asset "$asset_ref" "$repo" "$pattern")"
  mapfile -t asset_lines <<< "$asset_info"
  tag="${asset_lines[0]:-}"
  url="${asset_lines[1]:-}"
  asset="${asset_lines[2]:-}"
  [[ -n "$tag" && -n "$url" && -n "$asset" ]] || die "Could not resolve release asset for $program."

  log "Installing $program $tag from $repo into $bin_dir"
  log "Asset: $asset"

  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf 'DRY-RUN: download %s\n' "$url" >&2
    printf 'DRY-RUN: install binary %s to %s/%s\n' "$program" "$bin_dir" "$program" >&2
    return
  fi

  tmp_dir="$(mktemp -d)"
  archive="$tmp_dir/$asset"
  extract_dir="$tmp_dir/extract"
  mkdir -p "$extract_dir"
  download_file "$url" "$archive"

  # Nicht jedes Projekt packt seine Binary ein: osv-scanner etwa lädt sie blank
  # aus. Entschieden wird am Namen des Assets, nicht am Muster.
  case "$asset" in
    *.tar.gz|*.tgz)
      tar -xzf "$archive" -C "$extract_dir"
      ;;
    *)
      install -m 0755 "$archive" "$bin_dir/$program"
      rm -rf "$tmp_dir"
      return
      ;;
  esac

  installed=0
  shopt -s globstar nullglob
  for candidate in "$extract_dir"/**/"$program"; do
    if [[ -f "$candidate" ]]; then
      install -m 0755 "$candidate" "$bin_dir/$program"
      installed=1
      break
    fi
  done
  shopt -u globstar nullglob

  rm -rf "$tmp_dir"

  if [[ "$installed" -ne 1 ]]; then
    die "Release archive for $program did not contain a $program binary."
  fi
}
