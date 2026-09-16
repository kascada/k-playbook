---
description: Turn raw material — chat transcripts, notes, hand-offs — from k-playbook-local/inbox/, and during the switch to the knowledge store from the previous location material/, into topic documents under k-playbook-local/knowledge/extracted/, written through the knowledge tools, clustered by meaning and marked with their sources and confidence. Takes a file or directory as inbox/… or material/… (or without the prefix, resolved against both), or lists raw pieces, queue entries and material if no argument is given.
argument-hint: [inbox/<quelle>/<pfad> | material/<pfad> | <pfad>]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep, TodoWrite, mcp__k-playbook__k_playbook_knowledge_write, mcp__k-playbook__k_playbook_knowledge_read, mcp__k-playbook__k_playbook_knowledge_list, mcp__k-playbook__k_playbook_knowledge_search, mcp__k-playbook__k_playbook_knowledge_status, mcp__k-playbook__k_playbook_knowledge_inbox_list, mcp__k-playbook__k_playbook_knowledge_inbox_read, mcp__k-playbook__k_playbook_knowledge_inbox_put, mcp__k-playbook__k_playbook_knowledge_queue_list, mcp__k-playbook__k_playbook_knowledge_queue_add]
---

# k-docs-extract

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Raw material is the source side: a chat transcript, a hand-off note, a mail thread. It is
never indexed and it is never the knowledge itself. This command reads it, clusters it by
meaning and writes topic documents into the knowledge store, where `search` finds them.
It writes only through the knowledge tools — `k_playbook_knowledge_write` or
`k-playbook knowledge write` — with `producer: docs-extract`.

Produces:
- `k-playbook-local/knowledge/extracted/<NN>-<slug>.md` — one document per coherent topic,
  with `sources` pointing back at the raw pieces and a confidence line saying whether it
  was checked against the code. The header is composed by the tool.

Nothing else. No file under `knowledge/` is written past the tool, nothing lands in
`docs/extracted/` any more, and the raw material stays untouched — in `inbox/` as well as in
`material/`.

## Schritt 1 — Pfade und Zugriffsweg

From the context output:

- `INBOX_DIR = <local.dir>/inbox`, `INBOX_DISPLAY_PATH = k-playbook-local/inbox` — die
  Quelle. Gelesen wird der Eingang über das Werkzeug (`inbox_list`, `inbox_read`), nicht
  über das Dateisystem.
- `MATERIAL_DIR = <local.dir>/material`, `MATERIAL_DISPLAY_PATH = k-playbook-local/material`
  — **bisheriger Ort**, bis Schritt 5 der Umstellung übergangsweise mitgelesen (Tabelle
  „Transitional reads“ in `k-playbook/docs/knowledge-layout.md`). Gelesen mit Read und Glob.
- `EXTRACT_DIR = knowledge/extracted`, `EXTRACT_DISPLAY_PATH =
  k-playbook-local/knowledge/extracted` — das Ziel, **nur über das Werkzeug**. Pfade der
  Werkzeuge sind relativ zu `knowledge/`: `extracted/<NN>-<slug>.md`.
- `RESOLVED_DOCS_DIR = <local.dir>/docs` — nur zum Lesen, für Querverweise auf Doku, die
  noch dort liegt.

**Zugriffsweg**, in dieser Reihenfolge, der erste verfügbare gewinnt:

1. **MCP-Werkzeuge.** Bietet der Client `k_playbook_knowledge_write`, `…_read`, `…_list`,
   `…_inbox_list`, `…_inbox_read` und `…_queue_list` an, werden sie benutzt. `projectDir`
   ist `project.dir` aus dem Kontext.
2. **Subkommando.** Sonst `k-playbook knowledge …` über Bash, mit `--json`, **immer aus
   `project.dir`** als Arbeitsverzeichnis. Ein Aufruf unterhalb von `k-playbook-local/`
   findet eine falsche `K-PLAYBOOK.yaml`.

Beide Wege rufen dieselbe Fachlogik auf. Die Entsprechungen: `inbox_list` ↔
`knowledge inbox list`, `inbox_read <pfad>` ↔ `knowledge inbox read <pfad>`, `queue_list` ↔
`knowledge queue list`, `list` mit `kind: extracted` ↔ `knowledge list --kind extracted`,
`read <pfad>` ↔ `knowledge read <pfad>`, `write` ↔ `knowledge write <pfad> …` (Schritt 5).

**Pfade mit Zone.** Die Werkzeuge nennen Eingangspfade ohne Zone: `inbox_list` und das
Feld `origin` eines Queue-Eintrags sagen `chat/2026-09-12.md`. In `sources` steht der Pfad
relativ zu `k-playbook-local/` mit Zone: `inbox/chat/2026-09-12.md` oder
`material/befunde/x.md`. **Stelle `inbox/` voran**, bevor ein Eingangspfad nach `sources`
geschrieben oder mit `sources` verglichen wird — bei der Rückwärtssuche und beim Abgleich mit
der Queue. Zum Lesen über `inbox_read` wird das Präfix wieder abgenommen.

Command-specific policy:

- `INBOX_DIR` or `MATERIAL_DIR` missing: both are created by setup. Ask whether to create
  exactly that directory or to run `/k-gui`. Do not use a fallback path and do not abort
  hard. `inbox_list` on a missing inbox answers with an empty list; the question still
  applies.
- `knowledge/extracted/` is this command's own producer directory. It is created by the
  first `write`; missing is the normal state before the first run.
- **Raw material is read-only for this command.** Nothing in `INBOX_DIR` or `MATERIAL_DIR`
  is changed, moved, renamed or deleted — not after a successful extraction either, and
  nothing is copied from `material/` into `inbox/`. The material stays the source you can
  go back to.
- Write nothing but documents under `knowledge/extracted/`, and those only through the
  tool. The command creates no queue entry and puts nothing into the inbox. `docs/`,
  `AGENTS.md` and `opencode.json` belong to other producers.

## Schritt 2 — Quelle wählen

**Mit `$ARGUMENTS`:** the argument names exactly one file or one directory.

- Beginnt es mit `inbox/`: ein Pfad im Eingang. Er existiert, wenn `inbox_list` einen
  Eintrag mit genau diesem Pfad (ohne `inbox/`) nennt — dann eine Datei — oder Einträge
  darunter — dann ein Verzeichnis.
- Beginnt es mit `material/`: ein Pfad unter `MATERIAL_DIR`, geprüft am Dateisystem.
- **Ohne dieses Präfix** wird es gegen beide Wurzeln aufgelöst, wie oben. Genau ein Treffer
  → dieser wird genommen. Treffer an beiden Orten → den Nutzer fragen, welcher gemeint ist,
  und beide Pfade mit Zone nennen. So bedeutet `befunde` — der Aufruf aus `/k-danke` bis
  Schritt 5 — weiter `material/befunde/`.

- Kein Treffer: abbrechen und die aufgelösten Pfade nennen (`inbox/<arg>`,
  `material/<arg>`).
- Führt der Pfad aus beiden Wurzeln heraus (absolut, `..`): abbrechen. Rohmaterial gehört
  nach `INBOX_DISPLAY_PATH`; es wird nicht von außerhalb eingelesen und nicht dorthin
  kopiert — das Ablegen ist eine Nutzerentscheidung (`inbox_put`).
- Ist es ein Verzeichnis: alle Dateien darin sind die Quelle, als ein Vorgang.

**Ohne `$ARGUMENTS`:** zeige drei Gruppen getrennt:

1. **Eingang** — alle Einträge aus `inbox_list`, mit `inbox/` vorangestellt.
2. **Warteschlange** — die Einträge aus `queue_list` mit `target: extracted/`, je mit
   Kennung, `origin` (mit `inbox/` vorangestellt) und `reason`. Andere Ziele gehören
   anderen Erzeugern und erscheinen nicht. Ein `origin`, den `inbox_list` nicht kennt —
   eine Adresse draußen oder ein fehlendes Rohstück —, wird angezeigt, ist aber hier nicht
   auswählbar.
3. **Bisheriger Ort** — die Dateien unter `MATERIAL_DIR` (Glob), mit `material/`
   vorangestellt.

Markiere, was schon ausgewertet ist. Die Markierung kommt aus einer **Rückwärtssuche**:
`list` mit `kind: extracted`, danach `read` jedes Dokuments und seine `sources` aus dem Kopf
lesen, und gegen die Pfade mit Zone abgleichen. `docs/extracted/` wird dabei nicht gelesen.

```text
/k-docs-extract — Rohmaterial
─────────────────────────────────────
Eingang (k-playbook-local/inbox/)
  [x] inbox/chat/2026-07-02-abrechnung.md   → extracted/01-abrechnungslauf.md
  [ ] inbox/mail/uebergabe-routing.txt       noch nicht ausgewertet

Warteschlange (target extracted/)
  [ ] 20260912-143000-mail-uebergabe-routing-txt  inbox/mail/uebergabe-routing.txt — Routing-Entscheidungen festhalten

Bisheriger Ort (k-playbook-local/material/, bis Schritt 5 der Umstellung)
  [x] material/befunde/release.md            → extracted/02-release-waechter.md
  [ ] material/notizen-deployment.md         noch nicht ausgewertet

Welche Datei oder welches Verzeichnis soll ausgewertet werden?
```

Wait for the choice. A queue entry is chosen through its raw piece. Marked files may be
picked again — the run then adds to what is already there rather than duplicating it.

**Passende Queue-Einträge.** Nach der Wahl gleiche die gewählten Rohstücke (mit Zone) gegen
den `origin` der Einträge mit `target: extracted/` ab (mit `inbox/` vorangestellt). Jeder
Treffer ist ein Kandidat, den dieser Lauf erledigen kann (Schritt 5).

## Schritt 3 — Themen vorschlagen

Lies die Quelle: Rohstücke im Eingang über `inbox_read`, Dateien unter `MATERIAL_DIR` mit
Read. Cluster it **by meaning**, exactly as `/k-docs-code` does in its semantic scan — not
by the order of the transcript. A chat jumps back and forth; three passages about the same
subsystem belong in one topic, and one long passage may carry three.

Look for:

- **Entscheidungen** — was wurde festgelegt, mit welcher Begründung, was war die
  Alternative.
- **Betriebswissen** — wie etwas wirklich läuft, was beim letzten Vorfall passiert ist.
- **Verträge und Randbedingungen**, die im Code nicht sichtbar sind: Absprachen mit
  Dritten, Fristen, Grenzen einer externen Schnittstelle.
- **Offene Punkte** — was ausdrücklich unentschieden blieb.

**Nicht** aufnehmen: Ablaufplauderei, Terminabsprachen, alles was der Code selbst schon
sagt, und alles was in der Wissensablage oder unter `docs/code/` bereits richtig steht —
prüfe das mit `search`.

Show a compact topic list, not the documents:

```
Aus inbox/chat/2026-07-02-abrechnung.md schlage ich folgende Themen vor:

  abrechnungslauf        Warum der Lauf nachts läuft und was der Retry kostet
  routing-entscheidungen Entscheidung gegen Kafka, Begründung, Alternative
  offene-punkte          Was ausdrücklich unentschieden blieb

Bezug zu vorhandenem Wissen:
  - abrechnungslauf ergänzt extracted/01-abrechnungslauf.md
  - routing-entscheidungen widerspricht docs/code/02-architektur.md:34 („Events über Kafka")
```

## Schritt 4 — Bestätigungs-Gate

Ask once, bundled, per topic:

```
Pro Thema: neu / ergänzen / verwerfen?

  abrechnungslauf        → neu | ergänzen in extracted/<datei> | verwerfen
  routing-entscheidungen → neu | ergänzen in extracted/<datei> | verwerfen
  offene-punkte          → neu | ergänzen in extracted/<datei> | verwerfen
```

**Nichts schreiben vor der Bestätigung.** Kein „ich lege schon mal an", kein
Zwischenstand in der Ablage. Ohne Antwort auf dieses Gate endet der Command, ohne ein
Werkzeug schreibend aufgerufen zu haben.

„Ergänzen" heißt: das genannte Dokument wird mit dem Neuen **als Ganzes neu geschrieben**
(Schritt 5). Ziel darf nur ein Dokument unter `extracted/` sein, das `list` mit
`kind: extracted` nennt. Ein Dokument einer anderen Herkunft ist kein gültiges Ziel.

## Schritt 5 — Dokumente schreiben

Pro bestätigtem Thema ein `write` mit `producer: docs-extract`. Den Kopf baut das Werkzeug;
übergeben werden nur die Felder und ein Rumpf **ohne** `---`-Kopf.

**Felder:**

| Feld | Wert |
|---|---|
| `path` | neu: `extracted/<NN>-<slug>.md`; ergänzen: der Pfad des Ziels |
| `title` | die eigene Formulierung des Themas, einzeilig |
| `subject` | ein kurzer Begriff: das Thema des Dokuments |
| `origin` | `k-docs-extract, <Quelle(n) mit Zone>, <ISO-Datum>` — dieser Lauf, heutiges Datum aus `now.date` |
| `state` | immer `condensed`. `reviewed` bleibt einer Person oder einem Review vorbehalten |
| `format` | `markdown`, außer `inbox_list` nennt für das Rohstück `html`, `text` oder `pdf` (unter `material/` dieselbe Zuordnung nach Endung); bei Quellen verschiedener Formate `markdown` |
| `sources` | die Rohstücke mit Zone: `inbox/<quelle>/<pfad>`, `material/<pfad>` |
| `queue` | nur beim ersten geschriebenen Dokument, dessen `sources` den `origin` eines passenden Queue-Eintrags enthalten: dessen Kennung. Je `write` höchstens eine Kennung |

`<NN>` ist die nächste freie Nummer unter den Dokumenten, die `list` mit `kind: extracted`
nennt. Der Nummernkreis gilt nur innerhalb von `extracted/`.

**Rumpf:**

```markdown
# <Titel>

**Konfidenz:** unbestaetigt — Aussage aus dem Material

<Ein Satz: was dieses Dokument festhält.> <Ein Satz: worum es geht und aus welcher Quelle es stammt.>

## Was festgehalten wurde

<Die Aussage, in eigenen Worten geordnet — nicht der Mitschnitt.>

## Woran es hängt

<Begründung, Alternative, Randbedingung. Verweise auf Code als `pfad:zeile`, wenn
belegt.>

## Offene Fragen

> **Offen:** <Widerspruch oder Unklarheit, unaufgelöst stehen gelassen.>

## Verwandte Themen

- `extracted/<NN>-<slug>.md` — <in welchem Zusammenhang>
- `k-playbook-local/docs/code/<NN>-<slug>.md` — <solange diese Doku noch dort liegt>
```

Die Konfidenz-Zeile hat genau zwei Formen:

- `**Konfidenz:** unbestaetigt — Aussage aus dem Material` — Standard. Die Aussage stammt
  aus einem Chat und ist damit eine **Behauptung**, bis sie am Code geprüft ist.
- `**Konfidenz:** bestaetigt — belegt am Code` — die Aussage wurde in diesem Lauf am Code
  belegt; der Beleg steht als `pfad:zeile` im Rumpf.

**Ergänzen** schreibt das Dokument ganz neu: bestehendes Dokument über `read` holen, den Kopf
bis zum zweiten `---` abtrennen und `title`, `subject` und `sources` daraus übernehmen, das
Neue in die passenden Abschnitte einfügen und mit **vereinigten** `sources` per `write`
zurückschreiben. `origin` nennt diesen Lauf, `state` ist `condensed`. Die Konfidenz-Zeile
bleibt nur `bestaetigt`, wenn der bisherige und der neue Teil am Code belegt sind; sonst
`unbestaetigt`, und belegte Aussagen behalten ihr `pfad:zeile`.

**Regeln:**

- `sources` ist Pflicht. Ohne `sources` ist ein Auszug nicht nachprüfbar und damit wertlos.
- `bestaetigt` nur mit `pfad:zeile` im Rumpf.
- **Widerspricht ein Auszug dem Code, wird der Widerspruch als offene Frage vermerkt —
  nicht aufgelöst.** Weder wird die Chat-Aussage zurechtgebogen noch die Code-Doku
  „korrigiert". Beide Stände nennen, mit Fundstelle, und die Frage stehen lassen.
- Keine erfundenen Zusammenhänge. Was im Material nicht steht, steht auch nicht im
  Dokument. Bei Unklarheit fragen, gebündelt pro Dokument.
- Zitate sparsam und nur, wenn die genaue Formulierung die Aussage trägt. Ein Auszug ist
  kein Mitschnitt.

**Schreibweg.** Über MCP `k_playbook_knowledge_write` mit den Feldern oben und `body`. Über
Bash, aus `project.dir`, den Rumpf von stdin:

```bash
k-playbook knowledge write extracted/03-routing-entscheidungen.md \
  --producer docs-extract --title "Routing-Entscheidungen" --subject "Routing" \
  --origin "k-docs-extract, inbox/mail/uebergabe-routing.txt, 2026-09-16" \
  --state condensed --source inbox/mail/uebergabe-routing.txt \
  --queue 20260912-143000-mail-uebergabe-routing-txt --json <<'RUMPF'
# Routing-Entscheidungen

**Konfidenz:** unbestaetigt — Aussage aus dem Material
…
RUMPF
```

Statt stdin geht `--file <datei>` mit einer Datei außerhalb von `k-playbook-local/`. Jede
Quelle ist ein eigenes `--source`. Weist das Werkzeug ab, zeige die Meldung dem Nutzer und
schreibe dieses Dokument nicht auf anderem Weg.

Nach jedem geschriebenen Dokument kurz melden: Pfad und ob ein Queue-Eintrag damit erledigt
ist — nicht der Inhalt.

## Schritt 6 — Abschluss

Kompakte Zusammenfassung:

- Ausgewertete Rohstücke, mit Zone (`inbox/…`, `material/…`).
- Geschriebene Dokumente unter `EXTRACT_DISPLAY_PATH`, getrennt nach neu / ergänzt /
  verworfen.
- Erledigte Queue-Einträge mit Kennung; passende Einträge, die stehen blieben, mit Grund.
- Verteilung `bestaetigt` gegen `unbestaetigt`.
- Offene Fragen und Widersprüche zum Code, als Liste mit Fundstelle.
- Ausdrücklich: `INBOX_DISPLAY_PATH` und `MATERIAL_DISPLAY_PATH` sind unverändert — nichts
  verschoben, nichts gelöscht, nichts umbenannt, nichts kopiert.
- Auffindbar: die neuen Dokumente findet die Wissenssuche sofort (`k_playbook_knowledge_search`
  bzw. `k-playbook knowledge search`). Der Stichwort-Index `k-playbook-local/docs/README.md`
  kennt `knowledge/extracted/` bis zur Umstellung von `/k-docs-index` nicht; ein Lauf von
  `/k-docs-index` nimmt sie deshalb nicht auf.
- Folge-Command: keiner. Die Auswertung ist mit dem Schreiben abgeschlossen.

## Fehlerfälle

- `INBOX_DIR` oder `MATERIAL_DIR` fehlt → fragen, ob genau dieses Verzeichnis angelegt
  werden soll, oder `/k-gui` nennen. Kein Ersatzpfad, kein harter Abbruch.
- Eingang, passende Queue-Einträge und `MATERIAL_DIR` sind leer → melden, dass Rohmaterial
  nach `INBOX_DISPLAY_PATH` gehört (per Dateimanager oder `inbox_put`), und stoppen.
- `$ARGUMENTS` zeigt aus beiden Wurzeln heraus → abbrechen. Die Datei wird nicht eingelesen
  und auch nicht in den Eingang kopiert; das Ablegen ist eine Nutzerentscheidung.
- `$ARGUMENTS` ohne Präfix trifft an beiden Orten → fragen, nicht raten.
- **Binärstück im Eingang** → `inbox_read` liest nur die Textformate, die es an der Endung
  erkennt (`md`, `markdown`, `txt`, `html`, `htm`, `json`, `yaml`, `yml`, `csv`, `xml`,
  `log`), und weist alles andere ab — Bilder und PDFs ebenso wie Quelltext mit anderer Endung
  (`.ts`, `.mdx`). Melden, welches Rohstück, und überspringen; nicht am Werkzeug vorbei lesen
  und nicht raten, was drinsteht. Unter `MATERIAL_DIR` gilt dasselbe für Binärdateien und
  Archive.
- **Werkzeug fehlt** → der Client bietet die MCP-Werkzeuge nicht an und
  `k-playbook knowledge` meldet `unbekanntes Kommando`: die Installation ist älter als dieser
  Command. Melden, Update oder `/k-gui` nennen, und **nicht** von Hand nach
  `knowledge/extracted/` oder `docs/extracted/` schreiben.
- **Abweisung durch das Werkzeug** (`invalid_input`, etwa ein Rumpf mit Kopf, ein Pfad
  außerhalb von `extracted/`, ein abgelöstes Ziel, eine unbekannte Queue-Kennung) → die
  Meldung dem Nutzer zeigen. Nicht umgehen; korrigieren nur, wenn die Meldung den Fehler
  eindeutig benennt und die Korrektur am Inhalt nichts ändert, sonst fragen. Ein
  Queue-Eintrag bleibt bei einem gescheiterten `write` stehen.
- Der Nutzer bestätigt kein einziges Thema → nichts schreiben und das im Abschluss sagen.
- Ziel für „ergänzen" liegt außerhalb von `extracted/` → ablehnen und neu fragen.

## Anti-Muster (nicht tun)

- **Eine Datei unter `knowledge/` direkt schreiben** — mit Write, Edit oder der Shell, auch
  nicht „nur eine Zeile". Das Werkzeug ist der einzige Weg: es prüft Erzeuger und Pfad,
  baut den Kopf und erledigt den Queue-Eintrag erst, wenn das Dokument steht.
- **Nach `docs/extracted/` schreiben.** Das ist kein Ziel mehr; dort liegt nur, was vor der
  Umstellung entstand.
- **Mitschnitt umbenennen statt auswerten.** Ein Auszug, der dem Chatverlauf folgt, ist
  kein Dokument. Erst nach Bedeutung clustern, dann schreiben.
- **Behauptung als Tatsache.** Was im Chat gesagt wurde, ist unbestätigt, bis es am Code
  belegt ist. `**Konfidenz:** bestaetigt` ohne `pfad:zeile` ist eine Lüge im Dokument.
- **Widerspruch auflösen.** Wenn Auszug und Code sich widersprechen, ist das ein Befund
  für einen Menschen, keine Aufgabe für diesen Command.
- **Material aufräumen oder umziehen.** Verschieben, umbenennen, löschen oder nach `inbox/`
  kopieren nimmt die Quelle weg, gegen die man später prüfen würde. Der Umzug von
  `material/` gehört zu Schritt 5 der Umstellung, nicht zu einem Auswertungslauf.
- **Vor dem Gate schreiben.** Ein halb angelegtes Dokument aus einem abgebrochenen Lauf
  sieht später aus wie bestätigtes Wissen.
- **`inbox/` in `sources` weglassen.** Ohne Zone ist ein Eintrag mehrdeutig, und die
  Rückwärtssuche findet ihn nicht.
