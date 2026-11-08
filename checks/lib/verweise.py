#!/usr/bin/env python3
"""Verweise ohne Präfix in Commands, Instructions, Rules, Guidelines und Skills finden.

Maßgeblich ist k-playbook:/instructions/on-demand/verweise-nachziehen.md: Dort steht,
was ein Verweis ist und wie er nachgezogen wird. Dieser Check setzt nur den Teil
davon um, der am Text erkennbar ist; er meldet und korrigiert nichts. Seine Grenzen
sind bewusst:

- Gelesen werden die mitgelieferten Quellen unter <playbook>/defaults/ (commands/,
  instructions/, rules/, guidelines/) und <playbook>/share/ (commands/, skills/) und
  ihre projekteigenen Gegenstücke: commands/ und skills/ unter
  <config-root>/.k-playbook/, instructions/, rules/ und guidelines/ unter
  <config-root>/k-playbook-local/, jeweils alle .md- und .md.template-Dateien.
  Schlüssel ist der Pfad ab dem Playbook (defaults/…, share/…) bzw. ab der
  Konfigurationswurzel (.k-playbook/…, k-playbook-local/…). Den früheren Ort
  k-playbook-local/commands/ und k-playbook-local/skills/ liest der Check nicht mehr,
  wie das Programm seit Task 129. Die Tasks in der Projekt-Collection durchsucht der
  Check nicht.
- Gemeldet wird je Zeile und Art höchstens einmal:
  - ``link`` — ein relativer Markdown-Link, ``](../…)`` oder ``](./…)``, überall,
    auch in Codeblöcken (Vorlagen, die ein Command schreibt);
  - ``pfad`` — ein Pfad ohne Präfix in Backticks, der mit einer bekannten Wurzel
    beginnt (KNOWN_ROOTS), wenn er unter einer Überschrift ``Referenzen`` steht oder
    in einer Zeile mit Lese-Anweisung (lies, liest, lesen, siehe).
  Andere Backtick-Pfade meldet er nicht: Ob ein Pfad im Fließtext gelesen werden
  soll und welches von zwei Präfixen gemeint ist, sieht man am Text nicht.
- In umzäunten Codeblöcken (``` oder ~~~, erkannt wie in command_tool_guards.py)
  zählen Überschriften und ``pfad``-Funde nicht; ``link`` gilt weiter überall. Ein
  ``## Referenzen`` in einer Vorlage öffnet also keinen Bereich, ein ``# Kommentar``
  in einem Shell-Block beendet keinen.
- Geprüft wird immer der ganze Umfang, unabhängig von K_CHECK_MODE und
  K_CHECK_FILES_FROM. Nur so fällt ein veralteter Ausnahme-Eintrag auf.

Ausnahmen liegen ausschließlich in verweis-exceptions.tsv neben dem wirksamen
Check-Skript; dessen Verzeichnis kommt als erstes Argument.
"""
from __future__ import annotations

import fnmatch
import re
import sys
from dataclasses import dataclass
from pathlib import Path

from command_tool_guards import FENCE, PROJECT_DIR_NAME, config_root, resolve_dirs
from common import fail_technical, finish_findings, status


EXCEPTIONS_FILE = "verweis-exceptions.tsv"
EXCEPTIONS_HEADER = ["datei", "zeile", "art", "begründung"]
LOCAL_DIR_NAME = "k-playbook-local"
KINDS = ("link", "pfad")

PLAYBOOK_SCOPE = ("defaults/commands", "defaults/instructions", "defaults/rules",
                  "defaults/guidelines", "share/commands", "share/skills")
LOCAL_SCOPE = ("instructions", "rules", "guidelines")
# Projekteigene Commands und Skills liegen in .k-playbook/, nicht in k-playbook-local/.
PROJECT_SCOPE = ("commands", "skills")

# Bereiche, für die es keinen Eintrag je Bereich gibt: Hier steht jede Fundstelle
# einzeln in der Ausnahmeliste.
NO_AREA_SEGMENTS = {"commands", "instructions", "rules"}

KNOWN_ROOTS = ("k-playbook-local", "k-playbook", ".k-playbook", ".claude", ".opencode", "share", "defaults",
               "installer", "rules", "guidelines", "instructions", "docs", "checks", "bin",
               "commands", "skills", "tasks")

REL_LINK = re.compile(r"\]\((?:\.\./|\./)")
ROOT_PATH = re.compile(r"`(?:" + "|".join(re.escape(r) for r in KNOWN_ROOTS) + r")/[^`]*`")
READ_WORD = re.compile(r"\b(?:lies|liest|lesen|siehe)\b", re.IGNORECASE)
HEADING = re.compile(r"^[ \t]{0,3}(#{1,6})[ \t]+(.*?)[ \t#]*$")


# --- Dateien -----------------------------------------------------------------


def is_source(path: Path) -> bool:
    return path.name.endswith(".md") or path.name.endswith(".md.template")


def collect(directory: Path, key_prefix: str, files: dict[str, Path]) -> None:
    """Dateien rekursiv sammeln; Dotfiles und Symlinks sind nie Einträge."""
    try:
        entries = sorted(directory.iterdir())
    except OSError:
        return
    for entry in entries:
        if entry.name.startswith(".") or entry.is_symlink():
            continue
        key = f"{key_prefix}/{entry.name}"
        if entry.is_dir():
            collect(entry, key, files)
        elif is_source(entry):
            files[key] = entry


def source_files(playbook_dir: Path, local_dir: Path, project_dir: Path) -> dict[str, Path]:
    """project_dir ist <config-root>/.k-playbook/."""
    files: dict[str, Path] = {}
    for part in PLAYBOOK_SCOPE:
        collect(playbook_dir / part, part, files)
    for part in PROJECT_SCOPE:
        collect(project_dir / part, f"{PROJECT_DIR_NAME}/{part}", files)
    for part in LOCAL_SCOPE:
        collect(local_dir / part, f"{LOCAL_DIR_NAME}/{part}", files)
    return files


# --- Fundstellen -------------------------------------------------------------


def fence_closing(line: str) -> re.Pattern[str] | None:
    """Öffnet die Zeile einen umzäunten Codeblock, das Muster seiner Schlusszeile.

    Dieselbe Erkennung wie shell_blocks in command_tool_guards.py: ``` oder ~~~,
    mindestens drei Zeichen, beliebig eingerückt; eine Backtick-Zeile mit Backtick
    in der Info öffnet keinen Block. Geschlossen wird mit demselben Zeichen in
    mindestens gleicher Länge und sonst nichts in der Zeile; ohne Schlusszeile
    reicht der Block bis zum Dateiende.
    """
    match = FENCE.match(line)
    if not match:
        return None
    fence = match["fence"]
    if fence[0] == "`" and "`" in match["info"]:
        return None
    return re.compile(rf"^[ \t]*{re.escape(fence[0])}{{{len(fence)},}}[ \t]*$")


def scan(lines: list[str]) -> list[tuple[int, str, str]]:
    """Fundstellen als (Zeile, Art, Text) liefern.

    In einem umzäunten Codeblock zählen keine Überschriften und keine Funde der
    Art ``pfad``; ``link`` wird auch dort gemeldet.
    """
    found: list[tuple[int, str, str]] = []
    references_level = 0  # >0: unter einer Überschrift „Referenzen“ dieser Ebene
    closing: re.Pattern[str] | None = None  # gesetzt: innerhalb eines Codeblocks
    for number, line in enumerate(lines, start=1):
        if closing is not None:
            if closing.match(line):
                closing = None
            in_code = True
        else:
            closing = fence_closing(line)
            in_code = closing is not None
        if in_code:  # Codeblock samt Zaunzeilen: nur relative Links zählen
            if REL_LINK.search(line):
                found.append((number, "link", line.strip()))
            continue
        heading = HEADING.match(line)
        if heading:
            level = len(heading[1])
            if references_level and level <= references_level:
                references_level = 0
            if heading[2].strip() == "Referenzen":
                references_level = level
            continue
        if REL_LINK.search(line):
            found.append((number, "link", line.strip()))
        if ROOT_PATH.search(line) and (references_level or READ_WORD.search(line)):
            found.append((number, "pfad", line.strip()))
    return found


# --- Ausnahmen ---------------------------------------------------------------


@dataclass
class ExceptionEntry:
    datei: str
    zeile: int | None  # None: ein Eintrag für einen ganzen Bereich
    art: str
    source_line: int

    def covers(self, key: str, line: int, art: str) -> bool:
        if self.zeile is None:
            return fnmatch.fnmatchcase(key, self.datei + "*") and self.art in ("*", art)
        return (self.datei, self.zeile, self.art) == (key, line, art)


def read_exceptions(path: Path) -> tuple[list[ExceptionEntry], list[str]]:
    if not path.is_file():
        return [], []

    entries: list[ExceptionEntry] = []
    problems: list[str] = []
    seen: set[tuple[str, str, str]] = set()
    header_seen = False

    for number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if not raw.strip() or raw.lstrip().startswith("#"):
            continue
        fields = [value.strip() for value in raw.split("\t")]
        where = f"{path}:{number}"
        if not header_seen:
            if fields != EXCEPTIONS_HEADER:
                problems.append(f"{where}: invalid exception header, expected {'<TAB>'.join(EXCEPTIONS_HEADER)}")
                return [], problems
            header_seen = True
            continue
        if len(fields) != 4:
            problems.append(f"{where}: invalid exception, expected 4 tab-separated columns, got {len(fields)}")
            continue
        datei, zeile, art, begruendung = fields
        errors: list[str] = []
        area = datei.endswith("/")
        parts = Path(datei.rstrip("/")).parts
        if not datei:
            errors.append("datei is empty")
        elif datei.startswith("/") or ".." in parts:
            errors.append(f"datei '{datei}' is not a path below the playbook, .k-playbook/ or k-playbook-local/")
        elif area:
            if zeile != "*":
                errors.append("an area (datei ending in /) needs zeile '*'")
            if NO_AREA_SEGMENTS & set(parts):
                errors.append(f"area '{datei}' lies in commands/, instructions/ or rules/; list each finding")
        elif not (datei.endswith(".md") or datei.endswith(".md.template")):
            errors.append(f"datei '{datei}' is neither a .md file nor an area ending in /")
        elif not re.fullmatch(r"[0-9]+", zeile) or int(zeile) < 1:
            errors.append(f"zeile '{zeile}' is not a positive line number")
        if art not in KINDS and not (area and art == "*"):
            errors.append(f"art '{art}' is not one of {', '.join(KINDS)}" + (" or *" if area else ""))
        if not begruendung:
            errors.append("begründung is empty")
        if errors:
            problems.append(f"{where}: invalid exception: {'; '.join(errors)}")
            continue
        key = (datei, zeile, art)
        if key in seen:
            problems.append(f"{where}: duplicate exception {datei} {zeile} {art}")
            continue
        seen.add(key)
        entries.append(ExceptionEntry(datei, None if area else int(zeile), art, number))
    return entries, problems


# --- Ablauf ------------------------------------------------------------------


def main() -> int:
    try:
        return run()
    except Exception as exc:  # noqa: BLE001 — ein Absturz darf keinen Status vortäuschen
        return fail_technical(f"verweise: unexpected error: {exc!r}")


def run() -> int:
    if len(sys.argv) != 2:
        return fail_technical("usage: verweise.py <check-script-dir>")
    script_dir = Path(sys.argv[1]).resolve()
    playbook_dir, local_dir, _ = resolve_dirs(script_dir)
    project_dir = config_root() / PROJECT_DIR_NAME

    files = source_files(playbook_dir, local_dir, project_dir)
    if not files:
        status("skip", f"no sources found under {playbook_dir / 'defaults'}, {playbook_dir / 'share'}, "
                       f"{project_dir} or {local_dir}")
        return 0

    exceptions_path = script_dir / EXCEPTIONS_FILE
    try:
        exceptions, problems = read_exceptions(exceptions_path)
    except (OSError, UnicodeDecodeError) as exc:
        return fail_technical(f"{exceptions_path}: {exc}")

    findings: list[str] = []
    used: set[int] = set()
    for key, path in files.items():
        try:
            lines = path.read_text(encoding="utf-8").splitlines()
        except (OSError, UnicodeDecodeError) as exc:
            return fail_technical(f"{path}: {exc}")
        for line, art, text in scan(lines):
            cover = next((index for index, entry in enumerate(exceptions) if entry.covers(key, line, art)), None)
            if cover is not None:
                used.add(cover)
                continue
            what = "relative link" if art == "link" else "path without prefix where a reference is meant"
            findings.append(f"{path}:{line}: {what}: {text[:160]} (datei={key} zeile={line} art={art})")

    for index, entry in enumerate(exceptions):
        if index not in used:
            where = entry.datei if entry.zeile is None else f"{entry.datei}:{entry.zeile}"
            findings.append(
                f"{exceptions_path}:{entry.source_line}: stale exception: no {entry.art} finding at {where}"
            )
    findings.extend(problems)

    if used:
        ok_reason = f"{len(used)} known exception(s) matched; {len(files)} file(s) checked"
    else:
        ok_reason = f"no references without prefix; {len(files)} file(s) checked"
    return finish_findings(findings, ok_reason)


if __name__ == "__main__":
    raise SystemExit(main())
