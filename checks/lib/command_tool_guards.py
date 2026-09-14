#!/usr/bin/env python3
"""Werkzeugaufrufe in Commands ohne lokalen Guard oder Rückfall finden.

Der Check setzt den Abschnitt „Externe Werkzeuge" aus rules/command-authoring.md
durch. Er ist ein Hinweisgeber, kein Shell-Parser; seine Grenzen sind bewusst:

- Werkzeugliste ist allein die Spalte ``name`` aus scripts/base-tools.tsv.
- Gelesen wird der effektive Command-Katalog: mitgelieferte Commands aus
  <playbook>/commands/, projekteigene aus <config-root>/k-playbook-local/commands/.
  Vergleichseinheit ist der Pfad ab commands/; ein gleichnamiger lokaler Eintrag
  ersetzt den mitgelieferten, ein leerer schaltet ihn ab.
- Geprüft wird immer der ganze Katalog, unabhängig von K_CHECK_MODE und
  K_CHECK_FILES_FROM. Nur so fällt ein veralteter Ausnahme-Eintrag auf.
- Betrachtet werden umzäunte Codeblöcke mit der Sprachkennung bash, sh, shell
  oder zsh, dazu console mit ``$ ``-Prompt. Blöcke ohne oder mit anderer
  Kennung zählen nicht, Fließtext und Backticks ebenso wenig.
- Ein Aufruf ist ein Programmname am Befehlsanfang: am Zeilenanfang, nach ``;``,
  ``&&``, ``||``, ``&``, ``(`` sowie nach ``if``, ``elif``, ``then``, ``else``,
  ``do``, ``while``, ``until``, ``!`` und ``{``. Aufrufe nach einer Pipe, in
  Befehlsersetzungen oder hinter ``env``, ``sudo``, ``xargs``, ``command`` und
  ähnlichen Vorsätzen liegen außerhalb.
- Geschützt ist ein Aufruf nur im then-Zweig eines ``if command -v <werkzeug>``
  im selben Codeblock oder mit einem Rückfall ``||`` direkt hinter dem Aufruf
  (``<werkzeug> … || …``). Alles andere wird gemeldet.
- Ein offenes Konstrukt — Anführungszeichen, Backtick, ``$(`` oder ein Heredoc
  ohne Endmarke, auch ein ``<<``, das keins ist — macht den Rest des Blocks
  unauswertbar. Es wird als eigene Fundstelle gemeldet, statt die Aufrufe
  dahinter still zu übergehen.

Ausnahmen liegen ausschließlich in command-tool-guard-exceptions.tsv neben dem
wirksamen Check-Skript; dessen Verzeichnis kommt als erstes Argument.
"""
from __future__ import annotations

import bisect
import os
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path

from common import fail_technical, finish_findings, status


EXCEPTIONS_FILE = "command-tool-guard-exceptions.tsv"
EXCEPTIONS_HEADER = ["datei", "zeile", "werkzeug", "begründung"]
LOCAL_DIR_NAME = "k-playbook-local"
PLAYBOOK_DIR_NAME = "k-playbook"
SHELL_LANGUAGES = {"bash", "sh", "shell", "zsh"}
CONSOLE_LANGUAGE = "console"

FENCE = re.compile(r"^(?P<indent>[ \t]*)(?P<fence>`{3,}|~{3,})(?P<info>.*)$")
ASSIGNMENT = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*(\[[^\]]*\])?\+?=")
REDIRECTION_OPERATOR = re.compile(r"^[0-9]*(>>?|<|&>>?|>&|<&|>\|)$")
REDIRECTION = re.compile(r"^[0-9]*(>>?|<|&>>?|>&|<&|>\|)")


# --- Pfade -------------------------------------------------------------------


def resolve_dirs(script_dir: Path) -> tuple[Path, Path, bool]:
    """Playbook-Verzeichnis und projekteigenes Verzeichnis bestimmen.

    Der Runner übergibt das Playbook als K_CHECK_PLAYBOOK_DIR; dieser Wert gilt
    zuerst. Ohne ihn liegt ein mitgeliefertes Skript in checks/, dessen
    Elternordner das Playbook ist. Für einen Override unter
    k-playbook-local/checks/ bleibt dann nur die Annahme k-playbook/ neben
    k-playbook-local/. Der dritte Wert sagt, ob der Pfad so geraten wurde.
    """
    config_root = Path(os.environ.get("K_CHECK_CONFIG_ROOT") or os.getcwd()).resolve()
    local_dir = config_root / LOCAL_DIR_NAME
    given = os.environ.get("K_CHECK_PLAYBOOK_DIR")
    if given:
        return Path(given).resolve(), local_dir, False
    if script_dir == (local_dir / "checks").resolve():
        return config_root / PLAYBOOK_DIR_NAME, local_dir, True
    return script_dir.parent, local_dir, False


# --- Matrix ------------------------------------------------------------------


def read_matrix_tools(matrix: Path) -> set[str]:
    header: list[str] | None = None
    tools: set[str] = set()
    for raw in matrix.read_text(encoding="utf-8").splitlines():
        if not raw.strip() or raw.lstrip().startswith("#"):
            continue
        fields = raw.split("\t")
        if header is None:
            header = [value.strip() for value in fields]
            if "name" not in header:
                raise ValueError(f"{matrix}: header has no column 'name'")
            continue
        index = header.index("name")
        if index < len(fields):
            name = fields[index].strip()
            if name and name != "-":
                tools.add(name)
    return tools


# --- Command-Katalog ---------------------------------------------------------


def collect_commands(directory: Path, prefix: str, files: dict[str, Path]) -> None:
    """Command-Dateien rekursiv sammeln, wie die Registrierung es tut.

    README.md, Dotfiles und Symlinks sind nie Einträge.
    """
    try:
        entries = sorted(directory.iterdir())
    except OSError:
        return
    for entry in entries:
        name = entry.name
        if name.startswith(".") or entry.is_symlink():
            continue
        key = f"{prefix}/{name}" if prefix else name
        if entry.is_dir():
            collect_commands(entry, key, files)
        elif name != "README.md" and name.endswith(".md"):
            files[key] = entry


def is_empty_file(path: Path) -> bool:
    """Leer heißt: nichts außer Leerzeilen und Kommentaren."""
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return False
    return all(not line.strip() or line.strip().startswith("#") for line in text.splitlines())


def effective_commands(playbook_dir: Path, local_dir: Path) -> dict[str, Path]:
    shipped: dict[str, Path] = {}
    local: dict[str, Path] = {}
    collect_commands(playbook_dir / "commands", "", shipped)
    collect_commands(local_dir / "commands", "", local)

    result: dict[str, Path] = {}
    for key in sorted(set(shipped) | set(local)):
        if key in local:
            if is_empty_file(local[key]):
                continue
            result[key] = local[key]
        else:
            result[key] = shipped[key]
    return result


# --- Codeblöcke --------------------------------------------------------------


def strip_indent(line: str, width: int) -> str:
    removed = 0
    index = 0
    while index < len(line) and removed < width and line[index] == " ":
        index += 1
        removed += 1
    return line[index:]


def shell_blocks(lines: list[str]) -> list[list[tuple[int, str]]]:
    """Shell-Codeblöcke als Listen aus (Zeilennummer, Text) liefern."""
    blocks: list[list[tuple[int, str]]] = []
    index = 0
    while index < len(lines):
        match = FENCE.match(lines[index])
        if not match:
            index += 1
            continue
        fence = match["fence"]
        info = match["info"].strip()
        if fence[0] == "`" and "`" in info:
            index += 1
            continue
        language = info.split()[0].lower() if info else ""
        indent = len(match["indent"].expandtabs(4))
        closing = re.compile(rf"^[ \t]*{re.escape(fence[0])}{{{len(fence)},}}[ \t]*$")

        body: list[tuple[int, str]] = []
        cursor = index + 1
        while cursor < len(lines) and not closing.match(lines[cursor]):
            body.append((cursor + 1, strip_indent(lines[cursor], indent)))
            cursor += 1

        if language in SHELL_LANGUAGES:
            blocks.append(body)
        elif language == CONSOLE_LANGUAGE:
            blocks.append(console_commands(body))
        index = cursor + 1
    return blocks


def console_commands(body: list[tuple[int, str]]) -> list[tuple[int, str]]:
    """In console-Blöcken zählen nur Prompt-Zeilen und ihre Fortsetzungen."""
    result: list[tuple[int, str]] = []
    continued = False
    for number, text in body:
        stripped = text.lstrip()
        if stripped.startswith("$ "):
            command = stripped[2:]
        elif continued:
            command = text
        else:
            command = ""
        result.append((number, command))
        continued = bool(command) and command.rstrip().endswith("\\")
    return result


# --- Lexer -------------------------------------------------------------------


@dataclass
class Token:
    kind: str  # "word" oder "op"
    value: str
    line: int


class Lexer:
    """Zerlegt einen Codeblock in Wörter und Operatoren.

    Anführungszeichen, Befehlsersetzungen und Heredoc-Inhalte bleiben undurchsichtig;
    nur Operatoren außerhalb davon trennen Befehle.
    """

    def __init__(self, block: list[tuple[int, str]]) -> None:
        self.text = "".join(text + "\n" for _, text in block)
        self.numbers = [number for number, _ in block]
        self.starts: list[int] = []
        offset = 0
        for _, text in block:
            self.starts.append(offset)
            offset += len(text) + 1
        self.tokens: list[Token] = []
        self.word: list[str] = []
        self.word_line = 0
        self.heredocs: list[tuple[str, int]] = []
        # Offene Konstrukte, hinter denen der Rest des Blocks nicht auswertbar ist.
        self.unresolved: list[tuple[int, str]] = []

    def line_at(self, position: int) -> int:
        return self.numbers[bisect.bisect_right(self.starts, position) - 1]

    def unresolved_at(self, position: int, what: str) -> None:
        self.unresolved.append((self.line_at(position), what))

    def add(self, value: str, position: int) -> None:
        if not self.word:
            self.word_line = self.line_at(position)
        self.word.append(value)

    def flush(self) -> None:
        if self.word:
            self.tokens.append(Token("word", "".join(self.word), self.word_line))
            self.word = []

    def operator(self, value: str, position: int) -> None:
        self.flush()
        self.tokens.append(Token("op", value, self.line_at(position)))

    def last_char(self) -> str:
        return self.word[-1][-1] if self.word and self.word[-1] else ""

    def skip_backtick(self, start: int) -> int:
        text = self.text
        index = start + 1
        while index < len(text):
            if text[index] == "\\":
                index += 2
                continue
            if text[index] == "`":
                return index
            index += 1
        return len(text) - 1

    def skip_double(self, start: int) -> int:
        text = self.text
        index = start + 1
        while index < len(text):
            char = text[index]
            if char == "\\":
                index += 2
            elif char == '"':
                return index
            elif text.startswith("$(", index):
                index = self.skip_paren(index + 1) + 1
            elif char == "`":
                index = self.skip_backtick(index) + 1
            else:
                index += 1
        return len(text) - 1

    def skip_paren(self, start: int) -> int:
        text = self.text
        depth = 0
        index = start
        while index < len(text):
            char = text[index]
            if char == "\\":
                index += 2
                continue
            if char == "'":
                closing = text.find("'", index + 1)
                if closing == -1:
                    return len(text) - 1
                index = closing + 1
                continue
            if char == '"':
                index = self.skip_double(index) + 1
                continue
            if char == "(":
                depth += 1
            elif char == ")":
                depth -= 1
                if depth == 0:
                    return index
            index += 1
        return len(text) - 1

    def skip_heredocs(self, index: int) -> int:
        text = self.text
        for delimiter, line_number in self.heredocs:
            found = False
            while index < len(text):
                end = text.find("\n", index)
                if end == -1:
                    end = len(text)
                line = text[index:end]
                index = end + 1
                if line.strip() == delimiter:
                    found = True
                    break
            if not found:
                self.unresolved.append((line_number, f"heredoc without end marker '{delimiter}'"))
        self.heredocs = []
        return index

    def read_heredoc(self, index: int) -> int:
        text = self.text
        start = index
        index += 2
        if index < len(text) and text[index] == "-":
            index += 1
        while index < len(text) and text[index] in " \t":
            index += 1
        delimiter: list[str] = []
        while index < len(text) and text[index] not in " \t\n;&|()<>":
            if text[index] not in "'\"\\":
                delimiter.append(text[index])
            index += 1
        self.add(text[start:index], start)
        if delimiter:
            self.heredocs.append(("".join(delimiter), self.line_at(start)))
        return index

    def run(self) -> list[Token]:
        text = self.text
        index = 0
        while index < len(text):
            char = text[index]
            following = text[index + 1] if index + 1 < len(text) else ""
            if char == "\\":
                if following == "\n":
                    index += 2
                else:
                    self.add(text[index:index + 2], index)
                    index += 2
            elif char == "'":
                closing = text.find("'", index + 1)
                if closing == -1:
                    self.unresolved_at(index, "unclosed ' quote")
                    closing = len(text) - 1
                self.add(text[index:closing + 1], index)
                index = closing + 1
            elif char == '"':
                closing = self.skip_double(index)
                if text[closing] != '"':
                    self.unresolved_at(index, 'unclosed " quote')
                self.add(text[index:closing + 1], index)
                index = closing + 1
            elif char == "`":
                closing = self.skip_backtick(index)
                if text[closing] != "`":
                    self.unresolved_at(index, "unclosed ` command substitution")
                self.add(text[index:closing + 1], index)
                index = closing + 1
            elif char == "$" and following == "(":
                closing = self.skip_paren(index + 1)
                if text[closing] != ")":
                    self.unresolved_at(index, "unclosed $( command substitution")
                self.add(text[index:closing + 1], index)
                index = closing + 1
            elif char == "#" and not self.word:
                end = text.find("\n", index)
                index = len(text) if end == -1 else end
            elif char in " \t":
                self.flush()
                index += 1
            elif char == "\n":
                self.operator("\n", index)
                index += 1
                if self.heredocs:
                    index = self.skip_heredocs(index)
            elif char == ";":
                self.operator(";", index)
                index += 1
            elif char == "&":
                if following == "&":
                    self.operator("&&", index)
                    index += 2
                elif self.last_char() in ("<", ">"):
                    self.add("&", index)
                    index += 1
                elif following == ">":
                    self.add("&>", index)
                    index += 2
                else:
                    self.operator("&", index)
                    index += 1
            elif char == "|":
                if following == "|":
                    self.operator("||", index)
                    index += 2
                elif self.last_char() == ">":
                    self.add("|", index)
                    index += 1
                else:
                    self.operator("|", index)
                    index += 2 if following == "&" else 1
            elif char == "(":
                if self.last_char() in ("<", ">", "="):
                    closing = self.skip_paren(index)
                    if text[closing] != ")":
                        self.unresolved_at(index, "unclosed ( substitution")
                    self.add(text[index:closing + 1], index)
                    index = closing + 1
                else:
                    self.operator("(", index)
                    index += 1
            elif char == ")":
                self.operator(")", index)
                index += 1
            elif char == "<" and text.startswith("<<<", index):
                self.add("<<<", index)
                index += 3
            elif char == "<" and following == "<":
                index = self.read_heredoc(index)
            else:
                self.add(char, index)
                index += 1
        if self.heredocs:
            self.skip_heredocs(len(text))
        self.flush()
        return self.tokens


# --- Kontrollfluss -----------------------------------------------------------


@dataclass
class SimpleCommand:
    words: list[str]
    line: int
    piped: bool
    guards: frozenset[str]
    next_op: str | None = None


@dataclass
class Frame:
    state: str  # "cond", "then" oder "else"
    cond: list[SimpleCommand] = field(default_factory=list)
    negated: bool = False
    tools: frozenset[str] = frozenset()


def command_v_target(command: SimpleCommand) -> str | None:
    """Das eine Werkzeug aus ``command -v <werkzeug>``, sonst None."""
    words = command.words
    if len(words) < 3 or words[0] != "command" or words[1] != "-v":
        return None
    names: list[str] = []
    skip_next = False
    for word in words[2:]:
        if skip_next:
            skip_next = False
            continue
        if REDIRECTION_OPERATOR.match(word):
            skip_next = True
            continue
        if REDIRECTION.match(word):
            continue
        names.append(word)
    return names[0] if len(names) == 1 else None


def guard_tools(frame: Frame) -> frozenset[str]:
    """Werkzeuge, die eine if-Bedingung für ihren then-Zweig zusichert.

    Anerkannt wird nur eine Bedingung aus ``command -v <werkzeug>``-Aufrufen,
    verbunden mit ``&&``. Verneinung, Pipes und alles andere zählen nicht.
    """
    if frame.negated or not frame.cond:
        return frozenset()
    tools: set[str] = set()
    for position, command in enumerate(frame.cond):
        target = command_v_target(command)
        if target is None or command.piped:
            return frozenset()
        last = position == len(frame.cond) - 1
        if not last and command.next_op != "&&":
            return frozenset()
        if last and command.next_op not in (";", "\n"):
            return frozenset()
        tools.add(target)
    return frozenset(tools)


def simple_commands(tokens: list[Token]) -> list[SimpleCommand]:
    commands: list[SimpleCommand] = []
    stack: list[Frame] = []
    expect_command = True
    piped = False
    current: SimpleCommand | None = None

    for token in tokens:
        if token.kind == "op":
            if current is not None:
                current.next_op = token.value
                current = None
            piped = token.value == "|"
            expect_command = True
            continue

        word = token.value
        if current is not None:
            current.words.append(word)
            continue
        if not expect_command:
            continue

        top = stack[-1] if stack else None
        if word == "if":
            stack.append(Frame("cond"))
        elif word == "elif":
            if top is not None:
                top.state, top.cond, top.negated, top.tools = "cond", [], False, frozenset()
        elif word == "then":
            if top is not None and top.state == "cond":
                top.tools = guard_tools(top)
                top.state = "then"
        elif word == "else":
            if top is not None:
                top.state, top.tools = "else", frozenset()
        elif word == "fi":
            if stack:
                stack.pop()
            expect_command = False
        elif word == "!":
            if top is not None and top.state == "cond":
                top.negated = True
        elif word in ("do", "while", "until", "{"):
            pass
        elif word in ("done", "}"):
            expect_command = False
        elif ASSIGNMENT.match(word):
            pass
        else:
            guards: frozenset[str] = frozenset()
            for frame in stack:
                if frame.state == "then":
                    guards = guards | frame.tools
            current = SimpleCommand([word], token.line, piped, guards)
            commands.append(current)
            if top is not None and top.state == "cond":
                top.cond.append(current)
    return commands


def scan_blocks(lines: list[str], tools: set[str]) -> tuple[list[tuple[int, str]], list[tuple[int, str]]]:
    """Ungeschützte Aufrufe und nicht auswertbare Stellen je Datei liefern."""
    calls: list[tuple[int, str]] = []
    unresolved: list[tuple[int, str]] = []
    for block in shell_blocks(lines):
        lexer = Lexer(block)
        for command in simple_commands(lexer.run()):
            tool = command.words[0]
            if tool not in tools or command.piped:
                continue
            if tool in command.guards or command.next_op == "||":
                continue
            calls.append((command.line, tool))
        unresolved.extend(lexer.unresolved)
    return calls, unresolved


# --- Ausnahmen ---------------------------------------------------------------


@dataclass
class ExceptionEntry:
    datei: str
    zeile: int
    werkzeug: str
    source_line: int


def read_exceptions(path: Path, tools: set[str]) -> tuple[list[ExceptionEntry], list[str]]:
    if not path.is_file():
        return [], []

    entries: list[ExceptionEntry] = []
    problems: list[str] = []
    seen: set[tuple[str, int, str]] = set()
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
        datei, zeile, werkzeug, begruendung = fields
        errors: list[str] = []
        if not datei:
            errors.append("datei is empty")
        elif datei.startswith("/") or ".." in Path(datei).parts or not datei.endswith(".md"):
            errors.append(f"datei '{datei}' is not a path below commands/")
        if not re.fullmatch(r"[0-9]+", zeile) or int(zeile) < 1:
            errors.append(f"zeile '{zeile}' is not a positive line number")
        if not werkzeug:
            errors.append("werkzeug is empty")
        elif werkzeug not in tools:
            errors.append(f"werkzeug '{werkzeug}' is not in base-tools.tsv")
        if not begruendung:
            errors.append("begründung is empty")
        if errors:
            problems.append(f"{where}: invalid exception: {'; '.join(errors)}")
            continue
        key = (datei, int(zeile), werkzeug)
        if key in seen:
            problems.append(f"{where}: duplicate exception {datei} {zeile} {werkzeug}")
            continue
        seen.add(key)
        entries.append(ExceptionEntry(datei, int(zeile), werkzeug, number))
    return entries, problems


# --- Ablauf ------------------------------------------------------------------


def main() -> int:
    try:
        return run()
    except Exception as exc:  # noqa: BLE001 — ein Absturz darf keinen Status vortäuschen
        return fail_technical(f"command_tool_guards: unexpected error: {exc!r}")


def run() -> int:
    if len(sys.argv) != 2:
        return fail_technical("usage: command_tool_guards.py <check-script-dir>")
    script_dir = Path(sys.argv[1]).resolve()
    playbook_dir, local_dir, guessed = resolve_dirs(script_dir)

    matrix = playbook_dir / "scripts" / "base-tools.tsv"
    if not matrix.is_file():
        # Ein geratener Pfad darf den Check nicht still per skip abschalten.
        if guessed:
            return fail_technical(
                f"base tools matrix not found: {matrix}; the playbook directory of this "
                "override check was guessed, set K_CHECK_PLAYBOOK_DIR"
            )
        status("skip", f"base tools matrix not found: {matrix}")
        return 0
    try:
        tools = read_matrix_tools(matrix)
    except (OSError, UnicodeDecodeError, ValueError) as exc:
        return fail_technical(str(exc))

    commands = effective_commands(playbook_dir, local_dir)
    if not commands:
        status("skip", f"no command files found under {playbook_dir / 'commands'} or {local_dir / 'commands'}")
        return 0

    exceptions_path = script_dir / EXCEPTIONS_FILE
    try:
        exceptions, problems = read_exceptions(exceptions_path, tools)
    except (OSError, UnicodeDecodeError) as exc:
        return fail_technical(f"{exceptions_path}: {exc}")
    excepted = {(entry.datei, entry.zeile, entry.werkzeug) for entry in exceptions}

    findings: list[str] = []
    detected: set[tuple[str, int, str]] = set()
    for key, path in commands.items():
        try:
            lines = path.read_text(encoding="utf-8").splitlines()
        except (OSError, UnicodeDecodeError) as exc:
            return fail_technical(f"{path}: {exc}")
        calls, unresolved = scan_blocks(lines, tools)
        for line, tool in calls:
            call = (key, line, tool)
            # Ein Eintrag deckt genau einen Aufruf; ein zweiter desselben Werkzeugs
            # auf derselben Zeile bleibt eine Fundstelle.
            first = call not in detected
            detected.add(call)
            if first and call in excepted:
                continue
            findings.append(
                f"{path}:{line}: {tool}: call without local guard or fallback "
                f"(datei={key} zeile={line} werkzeug={tool})"
            )
        for line, what in unresolved:
            findings.append(f"{path}:{line}: {what}: rest of the shell block cannot be checked")

    for entry in exceptions:
        if (entry.datei, entry.zeile, entry.werkzeug) not in detected:
            findings.append(
                f"{exceptions_path}:{entry.source_line}: stale exception: "
                f"no unguarded {entry.werkzeug} call at {entry.datei}:{entry.zeile}"
            )
    findings.extend(problems)

    matched = len(excepted & detected)
    if matched:
        ok_reason = f"{matched} known exception(s) matched exactly; {len(commands)} command file(s) checked"
    else:
        ok_reason = f"no unguarded tool calls; {len(commands)} command file(s) checked"
    return finish_findings(findings, ok_reason)


if __name__ == "__main__":
    raise SystemExit(main())
