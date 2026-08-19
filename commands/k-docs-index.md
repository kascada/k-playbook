---
description: Build the single documentation index at k-playbook-local/docs/README.md from every origin — code/, libs/, extracted/, manual/ and unsorted flat files — check consistency, verify findability in at most two lookups, and register the docs in MEMORY (AGENTS.md + opencode.json). Takes no argument and always covers the whole docs directory.
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite]
---

# k-docs-index

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Builds the one index over everything that lives under `k-playbook-local/docs/`. This is
the only command that writes `docs/README.md`; every other docs command writes only into
its own subdirectory. There is no partial index — the command always reads all origins.

Produces:
- `k-playbook-local/docs/README.md` — introduction, overview per origin, „Libs & Stack"
  table, alphabetical keyword index, question→file mapping.
- `AGENTS.md` at project root — session-injected pointer to the docs directory.
- `opencode.json` (or `.jsonc`) at project root — registers `AGENTS.md` + the docs
  directory.

## Schritt 1 — Pfade auflösen

From the context output:

- `RESOLVED_DOCS_DIR = <local.dir>/docs`
- `DOCS_DISPLAY_PATH = k-playbook-local/docs`
- `CODE_DIR = <RESOLVED_DOCS_DIR>/code` — written by `/k-docs-code`
- `LIBS_DIR = <RESOLVED_DOCS_DIR>/libs` — written by `/k-docs-tools`
- `EXTRACTED_DIR = <RESOLVED_DOCS_DIR>/extracted` — written by `/k-docs-extract`
- `MANUAL_DIR = <RESOLVED_DOCS_DIR>/manual` — written by hand
- `INDEX_FILE = <RESOLVED_DOCS_DIR>/README.md`
- `AGENTS_FILE = <project.dir>/AGENTS.md`
- `OPENCODE_CONFIG = <project.dir>/opencode.json`, or `<project.dir>/opencode.jsonc` if
  that variant already exists. Never create both.

Derived paths for the index and for MEMORY registration:

- `DOCS_README_FROM_AGENTS = k-playbook-local/docs/README.md`
- `AGENTS_LINK_FROM_DOCS_README = ../../AGENTS.md`
- `DOCS_REFERENCE_PATH = ./k-playbook-local/docs`

Command-specific policy:

- If `RESOLVED_DOCS_DIR` or `MANUAL_DIR` is missing: ask whether to create exactly that
  directory or to run `/k-gui`. Do not use a fallback path and do not abort hard.
- `CODE_DIR`, `LIBS_DIR` and `EXTRACTED_DIR` are created by their producers on first run.
  A missing one is the normal state — skip that origin silently, do not create it and do
  not ask.
- This command writes exactly three files: `INDEX_FILE`, `AGENTS_FILE`,
  `OPENCODE_CONFIG`. It never writes a doc file into any subdirectory of
  `RESOLVED_DOCS_DIR`, and it never edits an existing doc file.
- `README.md` is never a doc entry — neither in `RESOLVED_DOCS_DIR` nor in a
  subdirectory. Subdirectory READMEs are short explanatory texts, not indexes.

## Schritt 2 — Bestandsaufnahme

Per origin, collect all `*.md` except `README.md`:

| Herkunft | Verzeichnis | Erzeuger |
|---|---|---|
| Code | `CODE_DIR` | `/k-docs-code`, Skill `ks-overlay-repo-analyse` |
| Libs | `LIBS_DIR` | `/k-docs-tools` |
| Extrahiert | `EXTRACTED_DIR` | `/k-docs-extract` |
| Manuell | `MANUAL_DIR` | Mensch |
| Unsortiert | flache `<RESOLVED_DOCS_DIR>/*.md` | kein Erzeuger |

Read the YAML frontmatter of every file: `title`, `description`, `tags`, `type`,
`generated.by`. For files under `LIBS_DIR` also `lib`, `version`, `severity` and
`last-reviewed` — exactly the fields the table in Schritt 5 is built from.

Show a compact overview per origin — count plus one line per file (`Datei — title`), not
the content:

```text
/k-docs-index — Bestandsaufnahme
─────────────────────────────────────
code/       <N> Dateien
libs/       <N> Dateien
extracted/  <N> Dateien
manual/     <N> Dateien
unsortiert  <N> Dateien (flach in docs/)
```

Numbering stays local per directory. There is no global numbering, so the same `NN-`
prefix may legitimately appear in two origins.

## Schritt 3 — Flache Dateien migrieren

Only if Schritt 2 found flat `<RESOLVED_DOCS_DIR>/*.md` besides `README.md`. These are
files from before the origin structure; they have no producer.

Show the list and offer to move them into `CODE_DIR`:

```text
Flache Doc-Dateien ohne Herkunft gefunden:
  00-overview.md
  01-stack.md
  ...

Nach docs/code/ verschieben? (ja/nein)
```

On „ja":

- Determine per file whether it is tracked: only if `project.vcs` says the project is
  under version control **and** `git ls-files --error-unmatch <datei>` succeeds, use
  `git mv`. Otherwise use plain `mv`. Do not call `git mv` blindly.
- Create `CODE_DIR` if it does not exist — that is `/k-docs-code`'s directory, and
  creating it here for the move is the one exception.
- If a file of the same name already exists in `CODE_DIR`: do not overwrite. Report the
  collision, leave the flat file where it is and continue.
- Report which files were moved with which command.

On „nein": build the index anyway and list those files under the origin „unsortiert".
Offer the move again on the next run.

**A leftover `libs/README.md` with an overview table:** there is exactly one index, and
the lib table now lives in `INDEX_FILE`. Ask once whether to reduce `LIBS_DIR/README.md`
to the short explanatory text, then do it only after confirmation:

```markdown
# Libs & Stack

Kuratierte Referenz zu den nicht-trivialen Libraries und Tools dieses Projekts.
Fokus: Pitfalls und Idiome — kein Ersatz für offizielle Doku.

Erzeugt von `/k-docs-tools`. Die Übersichtstabelle steht im Index unter
[`../README.md`](../README.md).
```

## Schritt 4 — Konsistenz prüfen

Check and **report**; do not repair silently and do not rewrite a doc file.

- **Fehlendes oder unvollständiges Frontmatter** — no `title` or no `description`. Such a
  file still goes into the index, with the file name as fallback title and a marker.
- **Tote Cross-Links** — relative Markdown links inside doc files that point to a file
  that does not exist. Name source file, line and target.
- **`generated.by` passt nicht zum Verzeichnis** — e.g. `generated: { by: k-docs-code }`
  in `EXTRACTED_DIR`. Every directory stands for an origin; only a value from a *foreign*
  origin is a finding. `CODE_DIR` (`docs/code/`) accepts `k-docs-code`, legacy
  `k-code2docs` and `ks-overlay-repo-analyse`; `LIBS_DIR` accepts `k-docs-tools` and
  legacy `k-tools-scan`.

Print the findings as a short list. If there are none, say so in one line. Ask nothing
here — the index is built either way; the user decides later what to fix and with which
producer.

## Schritt 5 — Index schreiben

Write `INDEX_FILE` with these blocks:

```markdown
# <Projektname> — Dokumentation

<Ein Absatz: was das Projekt ist, was in diesen Docs steht.>

> **Für AI-Sessions:** Diese Docs sind **autoritativ**. Nutze sie zuerst,
> bevor du Code liest. Siehe [`AGENTS.md`](<AGENTS_LINK_FROM_DOCS_README>) im Projekt-Root.

## Übersicht der Dokumente

### Code (`code/`) — aus dem Code abgeleitet

| Datei | Inhalt |
|-------|--------|
| [`code/00-overview.md`](code/00-overview.md) | ... |

### Extrahiert (`extracted/`) — erzeugt von `/k-docs-extract`

| Datei | Inhalt | Konfidenz |
|-------|--------|-----------|
| [`extracted/01-<slug>.md`](extracted/01-<slug>.md) | ... | bestaetigt |

### Handgepflegt (`manual/`)

| Datei | Inhalt |
|-------|--------|
| [`manual/<slug>.md`](manual/<slug>.md) | ... |

### Unsortiert — ohne Erzeuger

| Datei | Inhalt |
|-------|--------|
| [`<NN>-<slug>.md`](<NN>-<slug>.md) | ... |

## Libs & Stack

Kuratierte Referenz zu Libraries und Tools. Fokus: Pitfalls, nicht Tutorials.
Erzeugt von `/k-docs-tools`.

| Lib | Version | Severity | Letzter Review |
|-----|---------|----------|----------------|
| [fastapi](libs/fastapi.md) | 0.115.0 | high | 2026-07-12 |

## Stichwort-Index

Alphabetisch. Format: **Begriff** → `datei.md` §Abschnitt.

### A
- **<Begriff>** → `<herkunft>/<datei>.md` §<abschnitt>

### B
...

## Häufige Fragen → direkter Sprung

| Frage | Datei |
|-------|-------|
| Was macht das System insgesamt? | `code/00-overview.md` |
| Welchen Stack nutzt es? | `code/01-stack.md` |
| Wo werden Secrets verwaltet? | `code/04-config-und-secrets.md` |
| ... | ... |
```

**Regeln für den Aufbau:**

- Leave out the section for an origin that has no files — an empty table is noise.
- Every link is relative to `RESOLVED_DOCS_DIR` and carries the origin directory in the
  path.
- The „Libs & Stack" table is built from the frontmatter of the files in `LIBS_DIR`
  (`lib`, `version`, `severity`, `last-reviewed`), not from a separate libs index.

**Regeln für den Stichwort-Index:**

- Aufnehmen: **Domänenbegriffe, Fach-Vokabular, Env-Var-Namen, Feature-Namen, Bug-Namen,
  externe Systeme, zentrale Klassen/Modul-Rollen.**
- **Nicht** aufnehmen: generische Programmier-Wörter („Klasse", „Function", „loop"). Der
  Index ist **kein Grep**.
- Jeder Eintrag muss auf einen konkreten Abschnitt einer konkreten Datei zeigen (nicht
  nur „`api.md`").
- Beim Aufbau: für jede Doc-Datei die 3–10 Kernbegriffe extrahieren, dann alphabetisch
  mergen. Über alle Herkünfte hinweg, inklusive der Lib-Namen aus `LIBS_DIR`.

**Regeln für „Häufige Fragen":**

- Jede Frage ist eine, die ein neuer Entwickler oder eine neue AI-Session realistisch in
  Woche 1 stellt.
- Antwort ist **eine** Datei. Wenn die Antwort auf 3 Dateien verteilt ist, ist die Doku
  falsch geschnitten — dann strukturell nachbessern.

`INDEX_FILE` is generated in full on every run. If it already exists and carries
hand-written passages beyond the generated blocks, show a diff and let the user confirm
before writing.

## Schritt 6 — Verifikation

Interner Selbsttest, sichtbar für den User:

1. Wähle **3 nicht-triviale Konzepte, die nicht direkt aus einem Doc-Titel folgen**
   (z. B. eine spezielle Env-Var, ein interner Job-Name, ein bestimmter API-Endpunkt).
   Nimm sie aus verschiedenen Herkünften, wenn es mehrere gibt.
2. Für jedes: versuche über `INDEX_FILE` (Übersicht + Stichwort-Index + Q→Datei) in
   höchstens zwei Schritten zur Antwort zu kommen.
3. Schlägt eines der drei fehl: Index nachbessern — Stichwort ergänzen, Frage aufnehmen,
   Übersichtszeile schärfen — und den Test wiederholen. Doc-Dateien werden dabei nicht
   angefasst; ist die Doku falsch geschnitten, ist das ein Befund für den Erzeuger.

Ergebnis dem User zeigen. Erst weiter, wenn alle drei durchkommen.

## Schritt 7 — MEMORY registrieren

Der Kern dieses Schrittes: die Docs sind wertlos, wenn Folge-Sessions sie nicht
automatisch als autoritativ behandeln.

**7a — `AGENTS_FILE`:**

- Existiert nicht → aus
  `<playbook.dir>/skills/ai-session-memory/vorlagen/AGENTS.md.template` erzeugen und
  Platzhalter füllen (`<Projektname>`, „Was ist dieses Projekt?" aus der Übersichts-Doku
  ableiten, Themenbereiche aus den gefundenen Herkünften füllen, Kurzverweis-Tabelle aus
  dem „Häufige Fragen"-Block spiegeln). Die Pfade der Vorlage stimmen bereits mit
  `DOCS_DISPLAY_PATH` und `DOCS_README_FROM_AGENTS` überein — weicht ein aufgelöster Pfad
  davon ab, gilt der aufgelöste. Erwähne knapp, dass die Doc-Dateien normales Markdown mit
  OKF-kompatiblem YAML-Frontmatter sind; `README.md` bleibt der Einstieg.
- Existiert → prüfen, ob folgende Punkte enthalten sind: „Docs zuerst", Verweis auf
  `DOCS_README_FROM_AGENTS`, Ausnahmen-Regel. Fehlende oder auf einen alten Docs-Pfad
  zeigende Punkte **mit Bestätigung** einfügen/korrigieren. Rest unangetastet lassen.

**7b — `OPENCODE_CONFIG`:**

- Existiert nicht → aus
  `<playbook.dir>/skills/ai-session-memory/vorlagen/opencode.json.template` erzeugen.
  `references.docs.path` muss `DOCS_REFERENCE_PATH` sein. `description` **konkret**
  befüllen: Projektname + Liste der wichtigsten Themen + Hinweis auf
  `DOCS_README_FROM_AGENTS` als Index (nicht die Template-Platzhalter stehen lassen).
- Existiert → prüfen, ob `instructions` `AGENTS.md` enthält, ob `references.docs.path`
  nach Auflösung relativ zur Config-Datei auf `RESOLVED_DOCS_DIR` zeigt, und ob die
  `description` konkret ist. Fehlendes ergänzen, falsche/alte Docs-Pfade korrigieren,
  konkret machen — mit Bestätigung.

**7c — Restart-Hinweis:**

Explizit dem User sagen:

> OpenCode liest die Konfig einmal beim Start. Damit die neue Session-Memory greift,
> bitte OpenCode beenden (`/exit` oder Ctrl+C) und neu starten.

## Schritt 8 — Abschluss

Kompakte Zusammenfassung:

- Zahlen je Herkunft: `code/`, `libs/`, `extracted/`, `manual/`, unsortiert.
- Migration: wie viele flache Dateien verschoben, mit `git mv` oder `mv`, wie viele
  liegen geblieben.
- Konsistenz-Befunde aus Schritt 4 (Anzahl je Sorte, keine Wiederholung der Liste).
- Index: Anzahl Übersichts-Einträge, Stichwort-Einträge, Q→Datei-Einträge.
- Selbsttest: 3 von 3 bestanden, nach wie vielen Nachbesserungen.
- MEMORY: `AGENTS.md` (neu / ergänzt / unverändert), `opencode.json` (neu / ergänzt /
  unverändert).
- Restart-Hinweis.
- Folge-Command: **`/k-docs-extract`** — wenn unter `k-playbook-local/material/` noch
  nicht ausgewertetes Rohmaterial liegt, macht es daraus Docs unter `docs/extracted/`;
  danach diesen Command erneut laufen lassen. Liegt dort nichts, endet die Doku-Kette
  hier.

## Fehlerfälle

- `RESOLVED_DOCS_DIR` fehlt → fragen, ob genau dieses Verzeichnis angelegt werden soll,
  oder `/k-gui` nennen. Kein Ersatzpfad, kein harter Abbruch.
- Keine einzige Doc-Datei in irgendeiner Herkunft → melden, `/k-docs-code` nennen und
  stoppen. Ein Index über nichts ist kein Ergebnis.
- `opencode.json` **und** `opencode.jsonc` liegen beide vor → stoppen und den User
  entscheiden lassen, welche gilt. Nicht raten und nicht beide schreiben.
- Template unter `<playbook.dir>/skills/ai-session-memory/vorlagen/` fehlt → melden und
  die MEMORY-Registrierung überspringen; der Index ist davon unabhängig fertig.
- `git mv` schlägt fehl (Datei ungetrackt, Index gesperrt) → die Datei liegen lassen, den
  Fehler nennen, mit den übrigen weitermachen.

## Anti-Muster (nicht tun)

- **Doc-Dateien anfassen.** Dieser Command baut den Index. Wer eine Doc-Datei
  korrigiert, umgeht ihren Erzeuger und die Korrektur ist beim nächsten Lauf des
  Erzeugers weg.
- **Teil-Index.** Ein Index über nur eine Herkunft ist kein Index — die Auffindbarkeit
  entsteht gerade daraus, dass alle Herkünfte in einer Liste stehen.
- **Zweiter Index.** `libs/README.md` als Übersichtstabelle ist genau das. Es gibt einen
  Index, und der steht in `RESOLVED_DOCS_DIR`.
- **Grep-Ersatz-Index.** Jedes Wort aus dem Code als Stichwort → verwässert den Index
  unbrauchbar. Nur **Fachbegriffe** aufnehmen.
- **Befunde stillschweigend reparieren.** Fehlendes Frontmatter und tote Links werden
  gemeldet, nicht behoben — sonst weiß niemand, dass der Erzeuger etwas falsch macht.
- **`git mv` blind aufrufen.** Bei ungetrackten Dateien oder außerhalb eines Repos
  schlägt es fehl und die Migration bleibt halb erledigt.
- **Templates un-ausgefüllt schreiben.** `<Projektname>` und `<konkrete Themen>` im
  finalen `opencode.json` oder `AGENTS.md` sind ein Fehler.
