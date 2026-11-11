# k-docs-tools

Erzeugt kuratierte, **pitfall-orientierte** Steckbriefe zu den nicht-trivialen Libraries und
Tools des Projekts — Pitfalls, Auth- und Nebenläufigkeitsfallen, Hinweise zur Migration,
empfohlene Idiome; kein Tutorial, dafür gibt es die offizielle Doku. Welche Libraries und
Tools es gibt, sagt das Versionsinventar; dieser Command scannt keine Manifeste selbst.
Geschrieben wird nur in die Collection des Projekts, über ein lokales Arbeitsverzeichnis:
`k_playbook_docs_checkout` holt den Bestand, die Steckbriefe entstehen dort mit den
Datei-Werkzeugen, `k_playbook_docs_publish` gleicht zurück ab.

Produces:

- `keph:/docs/generated/libs/<name>.md` — ein Steckbrief je ausgewähltem Gegenstand des
  Inventars, geschrieben von `k_playbook_docs_publish`.

Dazu verschiebt der Lauf nach einem erfolgreichen `publish` die Doku-Aufträge aus dem
Preflight nach `keph:/queue/docs/done/` (Abschnitt „Doku-Aufträge“ der Docs-Ablage).

Nichts sonst. Die `README.md` des Verzeichnisses schreibt `publish` aus ihrer Vorlage; das
Arbeitsverzeichnis unter `.k-playbook/work/docs/libs/` entfernt `publish` nach einem
vollständigen Abgleich. Ausnahmen: Bestätigt der Nutzer im Abschluss, die im Preflight
ergänzten Ausschlüsse zu speichern, trägt `k_playbook_docs_exclude_add` sie in
`docs.exclude` von `K-PLAYBOOK.yaml` ein; bestätigt er in Schritt 1, das fehlende Inventar
zu erheben, schreibt `k-playbook inventory` nach `keph:/docs/generated/versions/inventory.md`.

**Abgrenzung zum Versionsinventar.** Das Inventar ist **vollständig und quellenorientiert**:
jede deklarierte Version aus jeder Quelle, nach Umgebung getrennt, ohne Auswahl. Die
Steckbriefe sind **kuratiert**: eine Auswahl, je Gegenstand ein Dokument, gelesen gegen die
Versionen, die `sources` nennt. Weicht das Inventar später ab, gibt das **Inventar** die
Auskunft; ein Steckbrief wird deswegen nicht aus dem Inventar umgeschrieben, sondern über
diesen Command nachgezogen, wenn sein Inhalt — nicht nur seine Versionszahl — veraltet ist.
Beide sprechen dieselbe Pin-Taxonomie: die aus dem Vertrag des Inventars,
`docs/version-inventory.md` der Installation, Abschnitt „Pin taxonomy“. Dieser Text
definiert keine eigene.

## Schritt 1 — Briefing, Inventar und Scope

include: _shared/erster-schritt.md

include: _shared/docs.md

Dieser Command ist der Schreiber des Erzeugers `libs`: sein Verzeichnis ist
`keph:/docs/generated/libs/`, seine Dokumente tragen `generated.by: k-docs-tools`.

Aus dem Briefing brauchst du `context.kephalaion` — `ok`, `address`, `message`. Ohne
`ok: true` endet der Command hier (siehe die Ablage oben).

**Das Inventar.** Lies `keph:/docs/generated/versions/inventory.md` mit `k_playbook_doc_read`
(ersatzweise `k-playbook read`). Aus dem Kopf nimmst du `generated.at` und die Zahl der
Einträge, `generated.stats.entries` — ein Inventar im alten Kopf trägt sie noch unter
`inventory.entries`.

- `not_found` — das Inventar fehlt. Biete an, `k-playbook inventory` jetzt laufen zu lassen
  (der Lauf ist deterministisch; Näheres in `k-playbook:/commands/k-doc-inventory.md`), und
  lies das Inventar danach neu. Lehnt der Nutzer ab oder scheitert der Lauf, endet der
  Command hier: ohne Inventar gibt es keinen Lauf und keinen eigenen Scan der Manifeste.
- Das Inventar ist da, aber leer (`0` Einträge) → das sagen, `/k-doc-inventory` nennen, und
  der Command endet hier.
- Der Kopf ist unvollständig oder nicht lesbar → das als Hinweis im Preflight nennen und
  anbieten, `k-playbook inventory` neu laufen zu lassen; der Rumpf bleibt lesbar.

**Der Scope** ist der aus dem Abschnitt „Scope“ der Docs-Ablage oben: `SCAN_ROOT`, Stand,
`EXTRA_PATH` aus `$ARGUMENTS`, die Ausschlüsse und `NEW_EXCLUDES`. Er begrenzt hier nur das
Zählen der Nutzung im Code (Schritt 4), nicht die Liste der Kandidaten.

Die offenen Doku-Aufträge mit `erzeuger: tools` suchst du wie im Abschnitt
„Doku-Aufträge“ der Ablage oben und merkst sie dir als `ORDERS`.

Zeige den Preflight und lass ihn **einmal** bestätigen; der Nutzer darf dabei Ausschlüsse
ergänzen (`NEW_EXCLUDES`, wie im Abschnitt „Scope“).

```text
/k-docs-tools — Preflight
─────────────────────────────────────
Collection    <context.kephalaion.address>
Ziel          keph:/docs/generated/libs/
Inventar      erhoben <generated.at>, <N> Einträge | Hinweis: <Problem im Kopf>
<die Scope-Zeilen aus dem Abschnitt „Scope“>
<die Zeilen „Aufträge“ aus dem Abschnitt „Doku-Aufträge“>
```

Bei „nein“ abbrechen; nichts wurde geholt oder geschrieben.

## Schritt 2 — checkout

Rufe `k_playbook_docs_checkout` mit `projectDir` = `context.project.dir` und
`erzeuger: libs` (ersatzweise `k-playbook docs checkout libs --json`).

- `state: checked_out` — das Arbeitsverzeichnis ist geholt. `WORK_DIR` ist `dir` aus der
  Antwort, `WORK_DISPLAY_PATH` derselbe Pfad ab `context.project.dir`
  (`.k-playbook/work/docs/libs`).
- `state: existing` — ein Arbeitsverzeichnis liegt schon da, etwa von einem
  abgebrochenen Lauf. Nichts wurde geholt oder überschrieben. Nenne `existing.since`,
  `existing.ageMinutes` und `existing.files` und frage: **weitermachen** (mit dem, was dort
  liegt) oder **verwerfen** (`k_playbook_docs_checkout` mit `discard: true`, holt neu)?
  Ohne Antwort geht es nicht weiter.
- `kephalaion_unavailable` — nichts geschrieben; der Command endet wie in der Ablage oben.

Geschrieben wird ab hier nur in `WORK_DIR`. Einzige Ausnahme ist der bestätigte Eintrag
über `k_playbook_docs_exclude_add` im Abschluss.

## Schritt 3 — Lesen

Lies **alle** Dokumente in `WORK_DIR`, bevor du die Kandidaten bildest. Je Dokument merkst
du dir Dateiname, `generated.by`, `generated.at`, `hints`, `questions` und die Einträge in
`sources`, deren `resource` das Inventar ist (`item`, `version`).

- `hints` befolgst du (Kopf in der Ablage oben) — etwa Schwerpunkte oder eine Zusatzquelle.
  Eine Zusatzquelle aus einem `hint` gehört zum Zählen wie im Abschnitt „Scope“.
- **Zuständig** ist dieser Lauf nur für Dokumente mit `generated.by: k-docs-tools` oder dem
  Altwert `k-tools-scan`. Alle anderen — ohne `generated.by` oder mit einem anderen Wert —
  bleiben im Arbeitsverzeichnis **unverändert liegen**, samt `hints` und `questions`.
- Ein Dokument gehört zu dem Gegenstand, den sein Inventar-Eintrag in `sources` als `item`
  nennt, sonst zu dem Gegenstand, aus dem sein Dateiname nach der Regel in Schritt 6
  entsteht. Den bisherigen Text nimmst du als Anregung, nicht als Vorlage zum Abschreiben.

## Schritt 4 — Kandidaten und Auswahl

**Kandidaten sind alle Gegenstände des Inventars.** Lies dazu den Rumpf des Inventars, die
Abschnitte je Umgebung (`## lokal`, `## dev`, `## devcontainer`, `## ci`, `## deployment`,
ohne Einträge `## Einträge`) mit den Spalten `Gegenstand`, `Art`, `Version`, `Pin`, `Scope`
und `Herkunft`. Ein Gegenstand (`<ökosystem>/<name>`, etwa `python/celery` oder
`go/github.com/yuin/goldmark`) steht dort je Quelle und Umgebung in einer eigenen Zeile, auch
mit verschiedenen Versionen. Als Kandidat fasst du ihn über seine Zeilen zusammen:

- **Art** — die Werte aus `Art` (`package`, `tool`, `runtime`, `image`, `chart`,
  `chart-dependency`, `action`).
- **Versionen** — die verschiedenen Werte aus `Version`, leere ausgelassen.
- **Pin** — die verschiedenen Werte aus `Pin`, nach der Taxonomie des Vertrags.
- **Scope** — die verschiedenen Werte aus `Scope` (`main`, `dev`, `optional`, `test`,
  `build`), dazu die Umgebungen, in denen er vorkommt.

Das Inventar führt die Angaben der Manifeste; aus Lockfiles kommen nur die direkten
Abhängigkeiten, transitive Pakete werden nicht ergänzt. Indirekte Angaben bringt nur
`go.mod` mit: Es nennt auch indirekte Go-Module (`// indirect`), und das Inventar führt sie
als gekennzeichnete Zeilen. Erkennungsmerkmal ist die Spalte `Herkunft`: Sie enthält
`indirekt (// indirect in go.mod)`. Abweichungen zwischen den Zeilen sind kein Fehler
dieses Commands; das Inventar weist sie selbst aus.

**Nutzung im Code** zählst du je Kandidat mit Grep, innerhalb des Scope aus Schritt 1 —
der Scope begrenzt nur dieses Zählen, nicht die Liste. Gesucht wird nach dem Import, passend
zur Sprache (Python `import <name>|from <name>`, JavaScript `require\(['"]<name>|from
['"]<name>`, Go `"<pfad>"` im `import`, …):

- `file_count` — wie viele Dateien importieren,
- `import_count` — Vorkommen insgesamt.

Bei Gegenständen ohne Import — `image`, `chart`, `chart-dependency`, `action`, `runtime` —
entfällt die Zählung; die Liste zeigt dort `—`.

**Score und Klasse.** Je Kandidat ein Score, transparent gehalten:

| Signal | Score |
|---|---|
| Framework oder Fundament, formt die Architektur (etwa FastAPI, Django, React, Celery, SQLAlchemy, NestJS, Rails) | +3 |
| Externe Kopplung mit Auth, Netz oder Zustand (Cloud-SDKs, DB-Treiber, Message-Broker, Auth-Anbieter, ML-Runtimes) | +3 |
| Steht auf der Known-Pitfall-Liste (unten) | +2 |
| Breit im Code (`file_count` ≥ 20 % aller Quelldateien im Scope) | +2 |
| Pin in allen Zeilen `exact` oder `digest` | +1 |
| Test, Lint oder Format ohne aufwendige Konfiguration (pytest, mypy, ruff, black, prettier, eslint) | −2 |
| Triviales Hilfsmittel (requests ohne Auth-Sonderfall, click, python-dotenv, chalk, colorama) | −2 |

- **A — empfohlen:** Score ≥ 3 oder auf der Known-Pitfall-Liste.
- **B — optional:** Score 1–2.
- **C — übersprungen:** Score ≤ 0.
- **Rein indirekt:** Ein Kandidat, dessen Zeilen alle das Merkmal
  `indirekt (// indirect in go.mod)` in der Spalte `Herkunft` tragen, kommt nach C — ohne
  Ausnahme, unabhängig vom Score und ohne Hinweis. Wer ihn will, holt ihn mit
  `add <gegenstand>` aus C.

**Known-Pitfall-Liste:** JWT, cryptography, aiohttp, pytz und Zeitzonen-Handling, Kafka,
LangChain, boto3, azure-sdk-*, pandas mit großen Frames, nebenläufige asyncio-Libraries, ORMs,
WebSocket-Libraries, gRPC. Sie ist eine Konstante dieses Commands: hier ergänzbar, nicht
projektweise abschaltbar.

**Vorhanden.** Hat ein Kandidat schon ein Dokument, für das dieser Lauf zuständig ist
(Schritt 3), steht er mit „vorhanden“ und dem bisherigen `generated.at` in der Liste.
Umschreiben ist eine Auswahl wie jede andere; vorhandene Kandidaten sind deshalb **nicht
vorausgewählt**, auch nicht in A.

Zeige die Liste **kompakt**: A und B einzeln, C als eine zusammengefasste Zeile.

```text
Kandidaten aus dem Inventar (<N> Gegenstände, erhoben <generated.at>):

Empfohlen (A):
  [x] python/fastapi            0.115.0          exact   in 47 Dateien
      → Framework, Reihenfolge der Middleware, Async-Fallen
  [ ] python/sqlalchemy         2.0.35           exact   in 28 Dateien   vorhanden (2026-10-02)
      → Session-Semantik 1.x → 2.0, Async-Unterschiede
  [x] container/postgres        16.4, 15         exact   —
      → Major-Upgrade braucht Dump/Restore, verschiedene Versionen je Umgebung

Optional (B):
  [ ] python/pydantic           2.9.2            exact   in 34 Dateien
  [ ] python/httpx              0.27.0           range   in 8 Dateien, mit Auth

Übersprungen (C): python/pytest, python/ruff, python/black, … (18 Gegenstände) — auf Wunsch
  ausklappen.

Ohne Gegenstand im Inventar: python-requests.md — bleibt liegen | drop

Kommandos:
  all             alle A und B aufnehmen
  A-only          nur A ohne „vorhanden“ (Standard)
  add <gegenstand>     aufnehmen, auch „vorhanden“ (umschreiben) oder aus C
  remove <gegenstand>  aus der Auswahl streichen
  show C          C vollständig zeigen
  drop <datei>    ein eigenes Dokument ohne Gegenstand im Inventar entfernen
  proceed         mit der aktuellen Auswahl weiter
```

- Ein eigenes Dokument, dessen Gegenstand nicht mehr im Inventar steht, zeigt die Liste
  unter „Ohne Gegenstand im Inventar“. Es bleibt liegen, außer der Nutzer wählt `drop`; hat
  es `hints`, braucht die Löschung die Bestätigung, dass sie mit ihm gehen (Kopf in der
  Ablage oben). `publish` deutet eine fehlende Datei als Löschung.
- Kein Hin und Her je Tool: Der Nutzer wählt in **einer** Nachricht, dann geht es weiter.
- Fällt alles in C und wählt der Nutzer nichts aus, sag das und ende mit dem Abschluss.
- **Nichts schreiben, bevor die Auswahl bestätigt ist.**

## Schritt 5 — Recherche

Setze zuerst `GENERATED_AT` wie in der Ablage oben (`date -Iseconds`). Recherchiere je
ausgewähltem Kandidaten, in Gruppen von etwa drei je Zwischenmeldung, damit der Nutzer den
Fortschritt sieht:

**Basis (immer):**

- Die Rolle im Projekt aus der Nutzung im Code und eigenem Wissen ableiten.
- Zwei bis drei Kernstellen im Code nennen, als `pfad:zeile`, innerhalb des Scope.
- Versionen und Pin aus Schritt 4 übernehmen.

**Klasse A — zusätzlich gezielt im Web:**

- `WebFetch` auf die Startseite der offiziellen Doku zur genutzten Version.
- `WebFetch` auf Changelog oder Release-Notes rund um die genutzte Version (major.minor).
- Höchstens ein gezielter Abruf nach bekannten Pitfalls an der offiziellen Quelle (etwa die
  Issues des Repos). Keine breite Suche: insgesamt **höchstens drei** Abrufe je Gegenstand.

**Klasse B — nur die Basis.** Ins Web nur, wenn beim Schreiben eine konkrete Frage aufkommt,
die nicht sicher zu beantworten ist.

Was sich aus Repo und Quellen nicht klären lässt, ist eine Unklarheit nach der Ablage oben:
sammeln und vor `publish` vorlegen, nicht raten. Bei vielen Kandidaten darfst du Recherche
und Schreiben auf Sub-Agenten verteilen, nach den Regeln der Ablage oben; jeder bekommt
zusätzlich seinen Kandidaten mit den Zeilen aus Schritt 4, den Scope, `WORK_DIR`, die
bisherige Fassung seines Dokuments samt `hints` und die Vorlage aus Schritt 6.

## Schritt 6 — Schreiben

Je ausgewähltem Kandidaten eine Datei `<WORK_DIR>/<name>.md`, mit den Datei-Werkzeugen.
`<name>` ist der Gegenstand, klein geschrieben; jedes Zeichen außer `a–z`, `0–9`, `.`, `_`,
`-` wird `-`. Aus `go/github.com/yuin/goldmark` wird `go-github.com-yuin-goldmark`, aus
`python/celery` wird `python-celery`. Ein vorhandenes Dokument, das umgeschrieben wird, behält
seinen Dateinamen. Keine `README.md`, keine anderen Dateien als `.md`.

Der Kopf ist der aus dem Abschnitt „Der Kopf“ der Ablage oben; die Vorlage zeigt, wie dieser
Command ihn belegt:

```markdown
---
title: "<Name, wie das Projekt ihn nennt>"
description: "Projektrelevante Pitfalls und Idiome für <Name>."
generated: { by: k-docs-tools, at: <GENERATED_AT> }
hints: [ <unverändert übernommen; das Feld fehlt, wenn das Dokument keine hatte> ]
questions: [ "<Frage, die genau dieser Steckbrief beantwortet>", … ]
sources:
  - resource: keph:/docs/generated/versions/inventory.md
    item: <gegenstand>
    version: <version>
  - resource: <URL der offiziellen Doku, wenn gelesen>
  - resource: <URL von Changelog oder Release-Notes, wenn gelesen>
---

# <Name>

## Rolle im Projekt

<1–2 Absätze: wofür es genutzt wird und wie zentral es für dieses Projekt ist. Zwei bis
drei Kernstellen als `pfad:zeile`.>

## Version und Migration

- Deklariert: `<version>` (`<pin>`) in <Umgebungen>; weitere Versionen je Umgebung, wenn
  das Inventar sie nennt.
- Bekannte Breaking Changes seit …: <knapp, mit Version>
- Upgrade-Pfad, nur wenn er für dieses Projekt zählt: …

## Pitfalls (projektrelevant)

- **<Kurztitel>** — <1–3 Sätze mit Ursache und Symptom>. Ref: `<pfad:zeile>` oder URL.

## Empfohlene Idiome

- <kurz>

## Anti-Patterns

- <kurz>

## Verwandte Themen

- `keph:/docs/generated/code/<NN>-<slug>.md` — <in welchem Zusammenhang>
- `<anderer-steckbrief>.md` — <in welchem Zusammenhang>
```

Regeln:

- **`sources` beginnt mit dem Inventar:** je Version, die das Inventar für den Gegenstand
  nennt, ein Eintrag mit `resource: keph:/docs/generated/versions/inventory.md`,
  `item: <gegenstand>` und `version: <version>`; `item` ist in allen diesen Einträgen
  gleich. Zeilen ohne Version geben keinen eigenen Eintrag. Nennt das Inventar für den
  Gegenstand gar keine Version, steht ein Eintrag ohne `version`. Danach folgen die
  Herstellerquellen, die du tatsächlich gelesen hast, mit ihrer URL — eine nicht gelesene
  URL steht nicht in `sources`.
- **Wie kritisch** ein Gegenstand für das Projekt ist, zeigen die Reihenfolge der Pitfalls
  (das Wichtigste zuerst) und ein Satz unter „Rolle im Projekt“ — kein Feld im Kopf.
- Pitfalls sind **konkret**, mit Ursache und Symptom, nicht allgemein („kann Fehler
  werfen“). Ist keiner bekannt, steht dort ehrlich: „Kein projektrelevanter Pitfall bekannt
  in dieser Version.“
- Keine Tutorials, keine „Getting Started“-Schnipsel, keine abgeschriebene Doku.
- Keinen Abschnitt „Offene Fragen“: Unklarheiten legst du wie in der Ablage oben vor
  `publish` vor; was offen bleibt, nennt der Abschluss.
- Verwandte Themen im selben Verzeichnis mit ihrem Dateinamen, die Code-Doku und andere
  Teile der Projektdoku als `keph:`-Verweis.
- Neue `hints` schlägst du nur vor; eingetragen werden sie erst nach Bestätigung, wie im
  Kopf beschrieben.

Nach jeder geschriebenen Datei kurz melden: Dateiname und Zeilenzahl, nicht der Inhalt.

## Schritt 7 — publish

Rufe `k_playbook_docs_publish` mit `projectDir` = `context.project.dir` und
`erzeuger: libs` (ersatzweise `k-playbook docs publish libs --json`). `allow_hints: true`
nur, wenn der Nutzer geänderte, neue oder mit einem Dokument entfallende `hints` bestätigt
hat.

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

Ist `open` leer und `cleaned` wahr, schließt du die Aufträge aus `ORDERS` ab, wie im
Abschnitt „Doku-Aufträge“ der Ablage oben; sonst bleiben alle offen.

## Schritt 8 — Abschluss

**Ausschlüsse speichern** wie im Abschnitt „Scope“ der Ablage, wenn der Nutzer im
Preflight `NEW_EXCLUDES` ergänzt hat.

Kompakte Zusammenfassung:

- Kandidaten aus dem Inventar: N Gegenstände, davon A = x, B = y, C = z; ausgewählt: k.
- Je Dokument der Zustand aus `publish`, gezählt nach neu, umgeschrieben, entfallen,
  unverändert; dazu die unberührten Dokumente anderer Schreiber.
- Geänderte `hints` (nur mit Bestätigung) und die offen gebliebenen Unklarheiten.
- Der Stand des Inventars (`generated.at`) und der beschriebene Stand des Codes aus
  Schritt 1 (Branch bzw. `<ref>` und Commit).
- Offene Dateien mit Grund, falls `publish` nicht aufgeräumt hat.
- Die Doku-Aufträge: abgeschlossen mit Verweis, offen geblieben mit Grund, oder keine.
- Ausdrücklich: geschrieben wurde nur nach `keph:/docs/generated/libs/`, ins
  Arbeitsverzeichnis `WORK_DISPLAY_PATH` und — beim Abschließen der Aufträge — nach
  `keph:/queue/docs/done/` — und, nur wenn der Nutzer es bestätigt hat, über
  `k-playbook inventory` nach `keph:/docs/generated/versions/inventory.md` und über
  `k_playbook_docs_exclude_add` nach `docs.exclude` in `K-PLAYBOOK.yaml`.
- Hinweis: Nach einem Versionssprung zieht zuerst das Inventar nach (`/k-doc-inventory`),
  danach dieser Command die Steckbriefe, deren Inhalt veraltet ist.
- Folge-Command: keiner. Den Index über `keph:/docs/` erzeugt das Indizieren, nicht ein
  Command.

## Fehlerfälle

- `context.kephalaion.ok` ist nicht `true` oder ein Werkzeug meldet
  `kephalaion_unavailable` → kein Lauf; `context.kephalaion.message` nennen und auf die
  Projekt-Collection auf der Setup-Seite (`/k-gui`) verweisen. Kein Rückfall auf
  `k-playbook-local/docs/libs/`.
- Das Inventar fehlt, und der Nutzer lässt es nicht erheben, oder `k-playbook inventory`
  scheitert → kein Lauf; die Meldung nennen und `/k-doc-inventory` als nächsten Schritt.
  Keine Manifeste selbst lesen.
- `$ARGUMENTS` fehlt auf der Platte, liegt außerhalb von `context.project.dir` oder in
  einem der immer ausgeschlossenen Verzeichnisse → abbrechen, den Pfad nennen (Abschnitt
  „Scope“ der Ablage).
- `k_playbook_docs_checkout` meldet `existing` → fragen: weitermachen oder verwerfen.
- `k_playbook_docs_checkout` ist unbekannt, `k-playbook docs` meldet `unbekanntes Kommando`
  oder `unbekannter Erzeuger "libs"` → die Installation ist älter als dieser Command:
  melden, `/k-upgrade` nennen, nichts von Hand nach Kephalaion schreiben.
- `k_playbook_docs_exclude_add` meldet `rejected`, `unsupported` oder ist unbekannt → wie im
  Abschnitt „Scope“ der Ablage.
- `WebFetch` scheitert oder liefert nichts Brauchbares → den Steckbrief aus eigenem Wissen
  schreiben, die Lücke als Unklarheit vorlegen, die URL nicht in `sources` eintragen.
- `publish` meldet `conflict`, `hints_changed` oder `failed` → wie in Schritt 7.

## Anti-Muster (nicht tun)

- **Manifeste selbst scannen.** Die Kandidaten kommen allein aus dem Inventar; fehlt es,
  wird es erhoben, nicht nachgebaut.
- **Kandidaten nach dem Scope filtern.** Der Scope begrenzt nur das Zählen im Code.
- **Abschreiben aus der offiziellen Doku.** Der Wert entsteht aus Pitfall und
  Projektkontext, nicht aus der Wiedergabe.
- **„Kann kaputtgehen, wenn X“.** Pitfalls sind konkret; gibt es keinen, steht das so da.
- **Web-Recherche für alle.** Nur für A, gezielt, höchstens drei Abrufe je Gegenstand.
- **Eigene Kopffelder.** Kein `lib`, `version`, `severity`, `last-reviewed`, `type`, `tags`
  oder `status` — Versionen stehen in `sources`, das Gewicht im Text.
- **Fremde Dokumente löschen oder umschreiben.** Was ein anderer Schreiber angelegt hat,
  bleibt unverändert im Arbeitsverzeichnis — `publish` löscht, was dort fehlt.
- **`hints` still ändern.** Neue nur nach Bestätigung, und dann mit `allow_hints`.
- **Konflikte still lösen.** Kein erneutes Schreiben über eine fremde Änderung hinweg,
  kein `discard` ohne Entscheidung des Nutzers.
- **Direkt in die Collection schreiben.** Nicht mit Kephalaion `write` oder `create` an
  `publish` vorbei.
- **`README.md` oder `index.md` schreiben.** Die README schreibt `publish`, den Index das
  Indizieren.
- **Auf die Platte ausweichen.** Ohne Kephalaion gibt es keinen Lauf;
  `k-playbook-local/docs/libs/` ist kein gültiger Ort mehr.
