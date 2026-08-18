---
description: Turn raw material from k-playbook-local/material/ — chat transcripts, notes, hand-offs — into topic docs under k-playbook-local/docs/extracted/, clustered by meaning and marked with their source and confidence. Takes a file or directory below material/, or lists what is there if no argument is given.
argument-hint: [material-datei-oder-verzeichnis]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite]
---

# k-docs-extract

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Raw material is the source side: a chat transcript, a hand-off note, a mail thread. It is
never indexed and it is never the documentation itself. This command reads it, clusters
it by meaning and writes topic docs the index can pick up. The index over these docs is
built by `/k-docs-index`; this command does not write it.

Produces:
- `k-playbook-local/docs/extracted/<NN>-<slug>.md` — one file per coherent topic, with
  `sources:` pointing back to the material and `confidence:` saying whether it was checked
  against the code.

Nothing else. This command writes only inside its own directory `docs/extracted/`, and it
never changes the material.

## Schritt 1 — Pfade auflösen

From the context output:

- `MATERIAL_DIR = <local.dir>/material`
- `MATERIAL_DISPLAY_PATH = k-playbook-local/material`
- `EXTRACT_DIR = <local.dir>/docs/extracted`
- `EXTRACT_DISPLAY_PATH = k-playbook-local/docs/extracted`
- `RESOLVED_DOCS_DIR = <local.dir>/docs` — nur zum Lesen, für Querverweise und für die
  Rückwärtssuche.

Command-specific policy:

- If `MATERIAL_DIR` is missing: ask whether to create exactly that directory or to run
  `/k-gui`. It is part of the structure that setting up creates. Do not use a fallback
  path and do not abort hard.
- `EXTRACT_DIR` is this command's own producer directory. Create it without asking if it
  is missing — before the first run that is the normal state.
- **`MATERIAL_DIR` is read-only for this command.** Nothing in it is changed, moved,
  renamed or deleted — not after a successful extraction either. The material stays the
  source you can go back to.
- Write nothing outside `EXTRACT_DIR`. `docs/README.md`, `AGENTS.md` and `opencode.json`
  belong to `/k-docs-index`; `docs/code/`, `docs/libs/` and `docs/manual/` belong to other
  producers.

## Schritt 2 — Quelle wählen

**Mit `$ARGUMENTS`:** the argument names exactly one file or one directory. Resolve it
against `MATERIAL_DIR` if it is relative.

- Existiert der Pfad nicht: abbrechen und den aufgelösten Pfad nennen.
- Liegt der Pfad nicht unter `MATERIAL_DIR`: abbrechen. Rohmaterial gehört nach
  `MATERIAL_DISPLAY_PATH`; es wird nicht von außerhalb eingelesen und nicht dorthin
  kopiert.
- Ist es ein Verzeichnis: alle Dateien darin sind die Quelle, als ein Vorgang.

**Ohne `$ARGUMENTS`:** list `MATERIAL_DIR` and mark what has already been extracted. The
marker comes from a backward search: read the `sources:` frontmatter of every
`<EXTRACT_DIR>/*.md` and match it against the file names.

```text
/k-docs-extract — Rohmaterial in k-playbook-local/material/
─────────────────────────────────────
  [x] 2026-07-02-chat-abrechnung.md    → extracted/01-abrechnungslauf.md
  [x] uebergabe-routing.txt            → extracted/02-routing-entscheidungen.md
  [ ] 2026-08-11-chat-auth.md          noch nicht ausgewertet
  [ ] notizen-deployment.md            noch nicht ausgewertet

Welche Datei soll ausgewertet werden?
```

Wait for the choice. Marked files may be picked again — the run then adds to what is
already there rather than duplicating it.

## Schritt 3 — Themen vorschlagen

Read the source and cluster it **by meaning**, exactly as `/k-code2docs` does in its
semantic scan — not by the order of the transcript. A chat jumps back and forth; three
passages about the same subsystem belong in one topic, and one long passage may carry
three.

Look for:

- **Entscheidungen** — was wurde festgelegt, mit welcher Begründung, was war die
  Alternative.
- **Betriebswissen** — wie etwas wirklich läuft, was beim letzten Vorfall passiert ist.
- **Verträge und Randbedingungen**, die im Code nicht sichtbar sind: Absprachen mit
  Dritten, Fristen, Grenzen einer externen Schnittstelle.
- **Offene Punkte** — was ausdrücklich unentschieden blieb.

**Nicht** aufnehmen: Ablaufplauderei, Terminabsprachen, alles was der Code selbst schon
sagt, und alles was in einer bestehenden Datei unter `docs/code/` bereits richtig steht.

Show a compact topic list, not the docs:

```
Aus <datei> schlage ich folgende Themen vor:

  01-abrechnungslauf        Warum der Lauf nachts läuft und was der Retry kostet
  02-routing-entscheidungen Entscheidung gegen Kafka, Begründung, Alternative
  90-offene-punkte          Was ausdrücklich unentschieden blieb

Bezug zu vorhandener Doku:
  - 01 ergänzt code/06-abrechnung.md
  - 02 widerspricht code/02-architektur.md:34 („Events über Kafka")
```

## Schritt 4 — Bestätigungs-Gate

Ask once, bundled, per topic:

```
Pro Thema: neu / ergänzen / verwerfen?

  01-abrechnungslauf        → neu | ergänzen in extracted/<datei> | verwerfen
  02-routing-entscheidungen → neu | ergänzen in extracted/<datei> | verwerfen
  90-offene-punkte          → neu | ergänzen in extracted/<datei> | verwerfen
```

**Nichts schreiben vor der Bestätigung.** Kein „ich lege schon mal an", kein
Zwischenstand auf der Platte. Ohne Antwort auf dieses Gate endet der Command, ohne eine
Datei angefasst zu haben.

„Ergänzen" heißt: in die genannte bestehende Datei unter `EXTRACT_DIR` einfügen, ihre
`sources:` um die neue Material-Datei erweitern, den Rest der Datei unangetastet lassen.
Eine Datei aus einer anderen Herkunft ist kein gültiges Ziel.

## Schritt 5 — Docs schreiben

Pro bestätigtem Thema eine Datei `<EXTRACT_DIR>/<NN>-<slug>.md`. Der Nummernkreis gilt
nur innerhalb von `EXTRACT_DIR`; er beginnt bei der nächsten freien Nummer dort und hat
nichts mit den Nummern in `docs/code/` zu tun.

```markdown
---
type: Extracted Knowledge
title: <Titel>
description: <Ein Satz: was diese Datei festhält.>
tags: [<kurze-tags>]
status: stable
generated: { by: k-docs-extract, at: <ISO-8601-datetime> }
sources:
  - resource: ../../material/<datei>
    title: <woher, in einem Halbsatz — Chat vom 2026-07-02, Übergabe von …>
confidence: bestaetigt | unbestaetigt
---

# <Titel>

<1-2 Sätze: worum es hier geht und aus welcher Quelle es stammt.>

## Was festgehalten wurde

<Die Aussage, in eigenen Worten geordnet — nicht der Mitschnitt.>

## Woran es hängt

<Begründung, Alternative, Randbedingung. Verweise auf Code als `pfad:zeile`, wenn
belegt.>

## Offene Fragen

> **Offen:** <Widerspruch oder Unklarheit, unaufgelöst stehen gelassen.>

## Verwandte Themen

- [../code/<NN>-<slug>.md](../code/<NN>-<slug>.md) — <in welchem Zusammenhang>
```

**Regeln:**

- `sources:` ist Pflicht und zeigt relativ auf die Material-Datei, aus der der Inhalt
  stammt. Bei mehreren Quellen mehrere Einträge. Ohne `sources:` ist ein Extrakt nicht
  nachprüfbar und damit wertlos.
- `confidence:` ist Pflicht und hat genau zwei Werte:
  - `unbestaetigt` — Standard. Die Aussage stammt aus einem Chat und ist damit eine
    **Behauptung**, bis sie am Code geprüft ist.
  - `bestaetigt` — die Aussage wurde in diesem Lauf am Code belegt; der Beleg steht als
    `pfad:zeile` in der Datei.
- `generated.by` ist immer `k-docs-extract`. Der Index prüft das gegen das Verzeichnis.
- **Widerspricht ein Extrakt dem Code, wird der Widerspruch als offene Frage vermerkt —
  nicht aufgelöst.** Weder wird die Chat-Aussage zurechtgebogen noch die Code-Doku
  „korrigiert". Beide Stände nennen, mit Fundstelle, und die Frage stehen lassen.
- Keine erfundenen Zusammenhänge. Was im Material nicht steht, steht auch nicht in der
  Datei. Bei Unklarheit fragen, gebündelt pro Datei.
- Zitate sparsam und nur, wenn die genaue Formulierung die Aussage trägt. Ein Extrakt ist
  kein Mitschnitt.
- Verwandte Themen verlinken, relativ und über Herkunftsgrenzen hinweg
  (`../code/…`, `../libs/…`, `../manual/…`).

Nach jeder geschriebenen Datei kurz melden, welche Datei geschrieben wurde (Dateiname +
Zeilen-Zahl, nicht Inhalt).

## Schritt 6 — Abschluss

Kompakte Zusammenfassung:

- Ausgewertete Material-Datei(en), mit Pfad unter `MATERIAL_DISPLAY_PATH`.
- Geschriebene Dateien in `EXTRACT_DISPLAY_PATH`, getrennt nach neu / ergänzt /
  verworfen.
- Verteilung `confidence: bestaetigt` gegen `unbestaetigt`.
- Offene Fragen und Widersprüche zum Code, als Liste mit Fundstelle.
- Ausdrücklich: `MATERIAL_DISPLAY_PATH` ist unverändert — nichts verschoben, nichts
  gelöscht, nichts umbenannt.
- Folge-Command: **`/k-docs-index`** — nimmt die neuen Dateien in den einzigen Index
  `k-playbook-local/docs/README.md` auf. Ohne diesen Lauf sind sie nicht verlinkt und für
  Folge-Sessions nicht auffindbar.

## Fehlerfälle

- `MATERIAL_DIR` fehlt → fragen, ob genau dieses Verzeichnis angelegt werden soll, oder
  `/k-gui` nennen. Kein Ersatzpfad, kein harter Abbruch.
- `MATERIAL_DIR` ist leer → melden, wohin Rohmaterial gehört, und stoppen.
- `$ARGUMENTS` zeigt außerhalb von `MATERIAL_DIR` → abbrechen. Die Datei wird nicht
  eingelesen und auch nicht nach `MATERIAL_DIR` kopiert; das Ablegen ist eine
  Nutzerentscheidung.
- Das Material ist kein Text (Binärdatei, Archiv, Bild) → melden, welche Datei, und
  überspringen. Nicht raten, was drinstehen könnte.
- Der User bestätigt kein einziges Thema → nichts schreiben und das im Abschluss sagen.
- Ziel für „ergänzen" liegt außerhalb von `EXTRACT_DIR` → ablehnen und neu fragen.

## Anti-Muster (nicht tun)

- **Mitschnitt umbenennen statt auswerten.** Ein Extrakt, der dem Chatverlauf folgt, ist
  kein Dokument. Erst nach Bedeutung clustern, dann schreiben.
- **Behauptung als Tatsache.** Was im Chat gesagt wurde, ist unbestätigt, bis es am Code
  belegt ist. `confidence: bestaetigt` ohne `pfad:zeile` ist eine Lüge im Frontmatter.
- **Widerspruch auflösen.** Wenn Extrakt und Code sich widersprechen, ist das ein Befund
  für einen Menschen, keine Aufgabe für diesen Command.
- **Material aufräumen.** Verschieben, umbenennen oder löschen nach der Auswertung
  nimmt die Quelle weg, gegen die man später prüfen würde.
- **Vor dem Gate schreiben.** Eine halb angelegte Datei aus einem abgebrochenen Lauf
  sieht später aus wie bestätigtes Wissen.
- **In `docs/code/` schreiben.** Auch wenn ein Extrakt dort thematisch hinpasst: das
  Verzeichnis gehört `/k-code2docs` und wird beim nächsten Lauf ohne Rücksicht neu
  geschrieben.
