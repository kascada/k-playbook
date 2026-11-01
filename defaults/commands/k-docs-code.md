# k-docs-code

Erzeugt aus dem Code eine kuratierte Projektdoku, die **Bedeutung** beschreibt, nicht
Oberfläche — ein Dokument je Thema, kein Ersatz für grep. Geschrieben wird nur in die
Collection des Projekts, über ein lokales Arbeitsverzeichnis: `k_playbook_docs_checkout`
holt den Bestand, die Dokumente entstehen dort mit den Datei-Werkzeugen,
`k_playbook_docs_publish` gleicht zurück ab.

Produces:

- `keph:/docs/generated/code/<NN>-<slug>.md` — ein Dokument je zusammenhängendem Thema,
  geschrieben von `k_playbook_docs_publish`.

Nichts sonst. Die `README.md` des Verzeichnisses schreibt `publish` selbst; das
Arbeitsverzeichnis unter `.k-playbook/work/docs/code/` entfernt `publish` nach einem
vollständigen Abgleich.

## Schritt 1 — Briefing und Scope

include: _shared/erster-schritt.md

include: _shared/docs.md

Dieser Command ist ein Schreiber des Erzeugers `code`: sein Verzeichnis ist
`keph:/docs/generated/code/`. Den zweiten Schreiber dort erkennst du an
`generated.by: ks-overlay-repo-analyse`.

Aus dem Briefing brauchst du:

- `context.kephalaion` — `ok`, `address`, `message`. Ohne `ok: true` endet der Command hier
  (siehe die Ablage oben).
- `context.project.dir`, `context.project.repoRoot`, `context.local.dir`,
  `context.playbook.dir`.

**Scope** — aus Briefing und Argument, nicht geraten:

- `SCAN_ROOT = context.project.repoRoot` — immer, nicht das aktuelle Verzeichnis.
- `$ARGUMENTS` ergänzt den Scope um **genau einen** Pfad, statt ihn zu ersetzen. Er muss
  existieren und innerhalb von `context.project.dir` liegen; relativ gilt er ab
  `context.project.dir`. Fehlt er oder liegt er außerhalb, abbrechen und den Pfad nennen —
  ein anderes Projekt wird dort dokumentiert, nicht von hier. `EXTRA_PATH` ist dieser Pfad,
  sonst leer.
- **Immer ausgeschlossen**, auch wenn das Argument hineinzeigt: `context.playbook.dir`
  (`k-playbook/`), `context.local.dir` (`k-playbook-local/`) und
  `<context.project.dir>/.k-playbook/`. Zeigt `$ARGUMENTS` in einen davon, abbrechen und
  sagen, dass diese Verzeichnisse nie zur Code-Doku gehören.
- **Weitere Ausschlüsse**, ohne Rückfrage: `.git/`, `venv/`, `.venv/`, `env/`,
  `node_modules/`, `dist/`, `build/`, `target/`, `_bases/`, `.next/`, `.nuxt/`,
  `__pycache__/`, `.pytest_cache/`, `.mypy_cache/`, `.ruff_cache/`, `coverage/`, `htmlcov/`,
  `.tox/`, `tests/fixtures/`, `**/*.min.js`, `**/*.lock`, `**/*.bundle.*` — dazu alles, was
  eine `.gitignore` unter `SCAN_ROOT` ausnimmt (Einträge als Glob-Muster, verschachtelte
  `.gitignore` nach bestem Vermögen).

Zeige den Preflight und lass ihn **einmal** bestätigen; der Nutzer darf dabei Ausschlüsse
ergänzen:

```text
/k-docs-code — Preflight
─────────────────────────────────────
Collection    <context.kephalaion.address>
Ziel          keph:/docs/generated/code/
Scope         <SCAN_ROOT> (project.repoRoot)
Zusätzlich    <EXTRA_PATH> | —
Immer außen   k-playbook/, k-playbook-local/, .k-playbook/
Ausschlüsse   <kompakte Liste der wirksamen Ausschlüsse>
```

Bei „nein“ abbrechen; nichts wurde geholt oder geschrieben.

## Schritt 2 — checkout

Rufe `k_playbook_docs_checkout` mit `projectDir` = `context.project.dir` und
`erzeuger: code` (ersatzweise `k-playbook docs checkout code --json`).

- `state: checked_out` — das Arbeitsverzeichnis ist geholt. `WORK_DIR` ist `dir` aus der
  Antwort, `WORK_DISPLAY_PATH` derselbe Pfad ab `context.project.dir`
  (`.k-playbook/work/docs/code`).
- `state: existing` — ein Arbeitsverzeichnis liegt schon da, etwa von einem
  abgebrochenen Lauf. Nichts wurde geholt oder überschrieben. Nenne `existing.since`,
  `existing.ageMinutes` und `existing.files` und frage: **weitermachen** (mit dem, was dort
  liegt) oder **verwerfen** (`k_playbook_docs_checkout` mit `discard: true`, holt neu)?
  Ohne Antwort geht es nicht weiter.
- `kephalaion_unavailable` — nichts geschrieben; der Command endet wie in der Ablage oben.

Geschrieben wird ab hier nur in `WORK_DIR`.

## Schritt 3 — Lesen

Lies **alle** Dokumente in `WORK_DIR`, bevor du den Code analysierst. Je Dokument merkst du
dir `generated.by`, `hints` und `questions` aus dem Frontmatter.

- `hints` sind der einzige von Hand gepflegte Teil eines erzeugten Dokuments: kurze Sätze,
  worauf dieser Lauf achten soll — etwa Zusatzquellen oder Schwerpunkte. Befolge sie.
- Den bisherigen Text nimmst du als Anregung, nicht als Vorlage zum Abschreiben.
- **Zuständig** ist dieser Lauf nur für Dokumente mit `generated.by: k-docs-code` (oder dem
  Altwert `k-code2docs`), deren Thema im Scope aus Schritt 1 liegt. Alle anderen — mit
  `generated.by: ks-overlay-repo-analyse`, ohne `generated.by` oder mit einem Thema außerhalb
  des Scans — bleiben im Arbeitsverzeichnis **unverändert liegen**, samt `hints` und
  `questions`.

## Schritt 4 — Analysieren und Struktur vorschlagen

**Keine Datei-für-Datei-Beschreibung.** Ergebnis dieses Schritts ist ein Verständnis nach
Bedeutung, nicht nach Verzeichnissen. Suche nach:

- **Fachlichen Subsystemen** — eigenständige Verantwortungsbereiche, gebündelt über
  Datenmodelle, Aufrufwege und gemeinsames Vokabular.
- **Querschnittsthemen** — Auth, Konfiguration und Secrets, Logging, Fehlerbehandlung,
  Persistenz, HTTP-Oberfläche, Hintergrundjobs, Feature-Flags, Wiederholungen und
  Zeitgrenzen.
- **Einstiegspunkten** — was startet was.
- **Externen Kopplungen** — Datenbanken, Queues, fremde APIs, Auth-Anbieter, Cloud-Dienste:
  wo sie im Code sitzen und wo sie konfiguriert sind.
- **Datenmodell** — zentrale Entitäten, wo definiert, wo umgeformt.
- **Kernlogik gegen Framework-Standard und Boilerplate** — nur Kernlogik verdient tiefe Doku.
- **Altlasten** — sichtbare Deprecations, TODOs, alte Muster, getrennt notiert.

Nicht aufnehmen: Framework-Standard ohne Eigenanteil, Kommentare, die den Code
nacherzählen, Signaturen einzelner Funktionen. Bei sehr großen Repos in Sub-Agenten
aufteilen (einer je Subsystem-Kandidat), jedem die Ausschlüsse und die Regel „nur
Bedeutung, keine Zeilenlisten“ mitgeben.

Zeige dann eine **kompakte** Liste, nicht die Doku selbst:

```text
Struktur für keph:/docs/generated/code/:

  neu          00-overview            Was das System tut, für wen, grober Ablauf
  umschreiben  02-architektur         Komponenten und wie sie zusammenspielen
  entfällt     07-altes-thema         <Grund>
  bleibt       20-overlay-delta       generated.by: ks-overlay-repo-analyse — nicht behandelt
  …

Aus dem Scan:  <2–3 Beobachtungen, die den Vorschlag begründen>
Nicht klar:    <1–2 Punkte, die Klärung brauchen>
Neue hints:    <Vorschläge je Dokument, oder keine>

OK / zusammenlegen / trennen / streichen / umbenennen?
```

- **entfällt** schlägst du nur für Dokumente vor, für die dieser Lauf zuständig ist
  (Schritt 3). Jedes andere steht unter **bleibt** und wird nicht angefasst — `publish`
  deutet eine fehlende Datei als Löschung.
- Ein Dokument, das umgeschrieben wird, behält seinen Dateinamen. Ein neues Thema bekommt
  eine freie Nummer im Arbeitsverzeichnis: Einer-Schritte mit bewussten Lücken (`00`, `01`,
  …, `90`), umsortiert wird nicht.
- **Nichts schreiben, bevor die Struktur bestätigt ist.**

## Schritt 5 — Schreiben

Je bestätigtem Thema eine Datei `<WORK_DIR>/<NN>-<slug>.md`, mit den Datei-Werkzeugen.
Dateinamen klein, nur `a–z`, `0–9`, `.`, `_`, `-`. Keine `README.md`, keine anderen
Dateien als `.md`.

```markdown
---
title: <Titel>
description: <Ein Satz: was dieses Dokument erklärt.>
generated: { by: k-docs-code, at: <ISO-8601-Zeitpunkt> }
hints: [ <unverändert übernommen; das Feld fehlt, wenn das Dokument keine hatte> ]
questions: [ "<Frage, die ein Neuer in der ersten Woche stellt>", … ]
---

# <Titel>

<1–2 Sätze: worum es geht, aus welcher Sicht.>

## Wozu

<Der fachliche oder technische Zweck.>

## Wie es funktioniert

<Die Kernidee: Datenfluss, Abhängigkeiten, Aufrufweg. Stellen im Code als `pfad:zeile`.>

## Wichtige Stellen im Code

- `pfad/zur/datei.go:45` — <was dort steht, in einem Satz>

## Randfälle

<Nicht Offensichtliches, versteckte Verträge, bekannte Fehler.>

## Verwandte Themen

- `<NN>-<slug>.md` — <in welchem Zusammenhang>
```

Regeln:

- `title` und `description` sind Pflicht; ohne sie lehnt `publish` ab und schreibt nichts.
- `hints` übernimmst du **unverändert**. Neue `hints` schlägst du nur vor (Schritt 4); erst
  nach Bestätigung trägst du sie ein. Dasselbe gilt, wenn ein bestätigt entfallendes
  Dokument `hints` hat: Seine Löschung braucht die Bestätigung, dass die `hints` mit ihm
  gehen.
- `questions` sind die Fragen, die genau dieses Dokument beantwortet — eine Frage, eine
  Datei. Verteilt sich eine Antwort über mehrere Dokumente, ist der Zuschnitt falsch.
- Wurden belastbare Quellen außer dem Code genutzt, nennt sie `sources:` mit mindestens
  `resource` je Eintrag. `verified:` trägt nur ein Mensch ein.
- Stellen im Code immer als `pfad:zeile`. Verwandte Themen im selben Verzeichnis mit ihrem
  Dateinamen, andere Teile der Projektdoku als `keph:`-Verweis — ein isoliertes Dokument ist
  ein Fehler.
- Keine erfundenen Erklärungen: Ist eine Stelle unklar, fragen statt raten, gebündelt je
  Dokument.
- Keine Fließtextwände; Sprache wie im Projekt vereinbart, in der Regel Deutsch.

Nach jeder geschriebenen Datei kurz melden: Dateiname und Zeilenzahl, nicht der Inhalt.

## Schritt 6 — publish

Rufe `k_playbook_docs_publish` mit `projectDir` = `context.project.dir` und
`erzeuger: code` (ersatzweise `k-playbook docs publish code --json`). `allow_hints: true`
nur, wenn der Nutzer in Schritt 4 geänderte, neue oder mit einem Dokument entfallende
`hints` bestätigt hat.

Melde die Antwort je Dokument: `created`, `written`, `deleted`, `unchanged`, `conflict`
oder `failed`, mit Revision; dazu `hintsChanged`, `readme` und ob aufgeräumt wurde
(`cleaned`).

- `invalid_input` — eine Ablehnung im Arbeitsverzeichnis (fehlendes Frontmatter, falscher
  Name, `README.md`, andere Datei als `.md`). Nichts wurde geschrieben. Die Stellen
  beheben und `publish` erneut aufrufen.
- `hints_changed` — `hints` weichen vom checkout ab, oder ein Dokument mit `hints` wird
  gelöscht oder neu angelegt. Nichts wurde geschrieben. Dem Nutzer die genannten Dokumente
  vorlegen; erst nach Bestätigung mit `allow_hints: true` erneut aufrufen, sonst die
  `hints` zurücksetzen.
- `conflict` — das Dokument wurde seit dem checkout in Kephalaion geändert, angelegt oder
  gelöscht; die eigene Fassung wurde nicht geschrieben. **Vorlegen, nicht still
  nachziehen.** Der Nutzer entscheidet: eigene Änderung verwerfen, oder die eigene Fassung
  sichern, mit `k_playbook_docs_checkout` und `discard: true` neu holen — was schon
  veröffentlicht ist, kommt dabei mit —, die Änderung auf den neuen Stand übertragen und
  erneut veröffentlichen.
- `failed` mit `kephalaion_unavailable` — beim Hub ist nichts angekommen. Später erneut
  `publish`; es schreibt dann nur noch den Rest.
- `failed` mit `outcome_unknown` — der Vorgang kann angekommen sein. **Nicht** noch einmal
  veröffentlichen, sondern mit Kephalaion `read` nachsehen und dem Nutzer berichten.

Arbeitsverzeichnis und Merkdatei bleiben liegen, solange etwas offen ist (`open`).

## Schritt 7 — Abschluss

Kompakte Zusammenfassung:

- Je Dokument der Zustand aus `publish`, gezählt nach neu, umgeschrieben, entfallen,
  unverändert; dazu die unberührten Dokumente anderer Schreiber.
- Geänderte `hints` (nur mit Bestätigung) und offene Fragen aus Schritt 5.
- Offene Dateien mit Grund, falls `publish` nicht aufgeräumt hat.
- Ausdrücklich: geschrieben wurde nur nach `keph:/docs/generated/code/` und ins
  Arbeitsverzeichnis `WORK_DISPLAY_PATH`.
- Folge-Command: keiner. Den Index über `keph:/docs/` erzeugt das Indizieren, nicht ein
  Command.

## Fehlerfälle

- `context.kephalaion.ok` ist nicht `true` oder ein Werkzeug meldet
  `kephalaion_unavailable` → kein Lauf; `context.kephalaion.message` nennen und auf die
  Projekt-Collection auf der Setup-Seite (`/k-gui`) verweisen. Kein Rückfall auf
  `k-playbook-local/docs/code/`.
- `$ARGUMENTS` fehlt auf der Platte, liegt außerhalb von `context.project.dir` oder in
  einem der immer ausgeschlossenen Verzeichnisse → abbrechen, den Pfad nennen.
- `k_playbook_docs_checkout` meldet `existing` → fragen: weitermachen oder verwerfen.
- `k_playbook_docs_checkout` ist unbekannt oder `k-playbook docs` meldet
  `unbekanntes Kommando` → die Installation ist älter als dieser Command: melden,
  `/k-upgrade` nennen, nichts von Hand nach Kephalaion schreiben.
- `publish` meldet `conflict`, `hints_changed` oder `failed` → wie in Schritt 6.
- Der Scan findet keine Kernlogik, nur Framework-Standard → das sagen und keine Doku
  erfinden.

## Anti-Muster (nicht tun)

- **Datei-für-Datei-Beschreibung.** „modul_x enthält Klasse Y“ steht schon im Code; die
  Doku beschreibt Bedeutung.
- **Themen zersplittern.** Dreißig winzige Dokumente statt zehn sinnvoller; eine Frage
  gehört in genau eine Datei.
- **Fremde Dokumente löschen oder umschreiben.** Was `ks-overlay-repo-analyse` geschrieben
  hat oder außerhalb des Scans liegt, bleibt unverändert im Arbeitsverzeichnis —
  `publish` löscht, was dort fehlt.
- **`hints` still ändern.** Sie sind der einzige von Hand gepflegte Teil; neue nur nach
  Bestätigung, und dann mit `allow_hints`.
- **Konflikte still lösen.** Kein erneutes Schreiben über eine fremde Änderung hinweg,
  kein `discard` ohne Entscheidung des Nutzers.
- **Direkt in die Collection schreiben.** Nicht mit Kephalaion `write` oder `create` an
  `publish` vorbei — die Regeln (Frontmatter, `hints`, Konflikte) setzt nur `publish`
  durch.
- **`README.md` oder `index.md` schreiben.** Die README schreibt `publish`, den Index das
  Indizieren.
- **Auf die Platte ausweichen.** Ohne Kephalaion gibt es keinen Lauf; `k-playbook-local/docs/`
  ist kein gültiger Ort mehr.
