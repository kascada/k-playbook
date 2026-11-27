# k-review

Führt ein einzelnes Review-Rezept gegen das Projekt aus. Ein Review ist ein Rezept auf
einen Scope; je nach Rezept läuft es auf einem von zwei Wegen:

- **Bericht** (`handoff` im Frontmatter des Rezepts): Erheben und Zusammenfassen. Das
  Rezept schreibt seine Funde als SARIF-Fundliste in ein Arbeitsverzeichnis, der Merge
  fasst sie zusammen, die Triage schreibt die Ergebnisdatei, und der Command legt sie in der
  Projekt-Collection unter `keph:/results/review/` ab. Am Code ändert sich nichts.
- **Interaktiv** (kein `handoff`): Das Rezept führt Stelle für Stelle durch den Code und
  ändert jede erst nach Freigabe. Es entsteht keine Fundliste und keine Ergebnisdatei.

Produces:

- Bericht: die Ergebnisdatei als `keph:/results/review/<YYYY-MM-DD>-<schlüssel>.md` in der
  Projekt-Collection — eine Tabelle mit einer Zeile je Bündel, nach Priorität sortiert, mit
  der Spalte „erledigt“, darunter jeder Fund im Detail; dort wird sie abgehakt. Im
  Arbeitsverzeichnis die Fundliste `<rezept>.sarif`, den Beleg `review-input.json` und
  `review-input.md` und `review-triage.md` als Zwischenstand der Ergebnisdatei.
- Interaktiv: Änderungen am Code, je Stelle erst nach Freigabe.

Nichts sonst: kein Log, keine Fälligkeit. Nach Kephalaion geht nur die Ergebnisdatei, nie
SARIF, `review-input.json` oder `review-input.md`. Das Arbeitsverzeichnis ist das zweite
Argument; ohne es legt das Programm ein neues unter `/tmp/` an, das nur der eigene Benutzer
lesen kann.

## Schritt 1 — Briefing

include: _shared/erster-schritt.md

## Schritt 2 — Rezept bestimmen

Die Rezepte stehen in `documents.reviews` des Briefings, je Eintrag `ref`, `title`,
`description` und `origin`; mitgeliefert und projekteigen sind dort schon gegeneinander
aufgelöst. Einträge mit `origin: disabled` sind abgeschaltet. Der Schlüssel eines Rezepts ist
der Dateiname ohne `review-` und ohne `.md`: `k-playbook:/reviews/review-tech.md` hat den
Schlüssel `tech`.

`$ARGUMENTS` trägt bis zu zwei Teile: zuerst den Namen des Rezepts, danach den Pfad des
Arbeitsverzeichnisses. Ein relativer Pfad gilt ab `project.dir`.

**Mit Namen.** Streife ein führendes `review-` und ein abschließendes `.md` ab und suche
den Schlüssel in `documents.reviews`.

- Abgeschaltet (`origin: disabled`): Sag, dass das Projekt dieses Rezept abschaltet, statt
  es als unbekannt zu melden.
- Nicht vorhanden: Zeig die Auswahl unten und lass wählen.
- Passt ein Teilname auf mehrere Rezepte: Zeig die Kandidaten und frag nach dem genauen
  Namen.

**Ohne Namen.** Lies von jedem nicht abgeschalteten Rezept den Kopf mit
`k_playbook_doc_read` (`ref` des Eintrags, `lines: "1-80"`; endet das Frontmatter dort
nicht, lies weiter) und zeig nur die Rezepte, die für `/k-review` aktiviert sind (unten):

```
Verfügbare Reviews:

  [D] code                     — Code-Review (Bericht)
  [D] python-comment-hardspots — Python - Nicht-rekonstruierbare Entscheidungen kommentieren (interaktiv)
  [D] tech                     — Tech-Debt-Analyse (Bericht)

  D = mitgeliefert, P = projekteigen, Ü = überlagert eine mitgelieferte Fassung

Welches Review ausführen?
```

`D`, `P` und `Ü` stehen für `origin` `dist`, `local` und `override`. Warte auf die Wahl.

**Rezept lesen.** Lies das gewählte Rezept ganz mit `k_playbook_doc_read`. Aus dem
Frontmatter gelten:

- `title`, `scope-hint` (Freitext, kann fehlen), `language` (optional),
- `review.enabled` — fehlt der Block, gilt `true`. Steht er auf `false`, brich ab: Dieses
  Rezept ist für `/k-review` nicht aktiviert; es läuft höchstens im Audit.
- `handoff` — gesetzt heißt Bericht, fehlt es, interaktiv. Den Wert selbst übernimmt dieser
  Command nicht als Folge-Command.
- für den Bericht der Evidence-Vertrag: `audit.mode: evidence`, `audit.ruleIds` und
  `audit.scope.paths`. Fehlt davon etwas, brich ab und nenne das Rezept und das fehlende
  Feld: ohne diesen Vertrag kann das Rezept keine Fundliste erheben.

`audit.enabled`, `interval-weeks` und `result-family` wirken hier nicht.

## Schritt 3 — Scope klären

Zeig den `scope-hint` des Rezepts, falls vorhanden, und lass den Nutzer den Scope bestätigen
oder eingrenzen:

- Welche Dateien oder Verzeichnisse sollen geprüft werden?
- Soll etwas ausdrücklich ausgeschlossen werden?

Fahre erst fort, wenn der Scope klar ist. Im Bericht prüft das Rezept nur, was zugleich in
`audit.scope.paths` liegt; nenne das, wenn der gewählte Scope darüber hinausgeht.

## Schritt 4 — Bericht

### Arbeitsverzeichnis

Rufe `k_playbook_review_workdir` mit `projectDir` = `project.dir`, `recipe` = Schlüssel und,
wenn ein Pfad übergeben wurde, `dir` = dieser Pfad absolut; ersatzweise in der Shell
`k-playbook review workdir <schlüssel> [--dir <pfad>]`. Das Programm legt das Verzeichnis an
— ohne Pfad ein neues unmittelbar unter `/tmp/` mit Projekt, Rezept und Zeitstempel im
Namen, nur für den eigenen Benutzer lesbar — und antwortet mit `dir`. Das ist `WORK_DIR`.
Lege das Verzeichnis nie selbst an.

Steht in der Antwort `insideProject: true` oder eine andere Warnung, gib sie dem Nutzer
weiter: Ein Arbeitsverzeichnis im Projekt lesen Scanner späterer Läufe mit, solange es dort
liegt, und die Rohdaten eines Reviews können Secrets enthalten.

### Erheben

Mit `RECIPE_KEY` = Schlüssel des Rezepts, `SCOPE` = Scope aus Schritt 3, `WORK_DIR` wie oben:

include: _review-run/erheben.md

### Zusammenfassen

Mit `WORK_DIR` wie oben; der Kopf der Ergebnisdatei ist das Rezept (`review-<schlüssel>`),
der Scope aus Schritt 3 in Worten und `now.date`, auf dem Weg von `/k-review` (mit
Frontmatter `title`):

include: _review-run/zusammenfassen.md

### Ablegen

Nach dem Zusammenfassen legt der Command die Ergebnisdatei in der Projekt-Collection ab, bei
jedem Bericht-Lauf und ohne Rückfrage. Die Regeln für `keph:/results/` stehen in
`k-playbook:/instructions/on-demand/ergebnisse.md`; hier gilt:

- **Name:** `results/review/<now.date>-<schlüssel>.md`, etwa `results/review/2026-11-27-tech.md`.
- **Inhalt:** genau `WORK_DIR/review-triage.md`, mit dem Frontmatter `title` aus der Triage.
  Fehlt es, ergänze es vor dem Ablegen in der Datei im Arbeitsverzeichnis: in doppelten
  Anführungszeichen, Rezept, Datum und Scope in einem Satz.
- **Anlegen** mit Kephalaion `create`, `collection` = `kephalaion.address`, `name` wie oben,
  `content` der Inhalt. Antwortet es mit `name_taken`, hänge `-2`, `-3` … an den Kurznamen
  (`2026-11-27-tech-2.md`) und versuche es erneut, bis der Name frei ist. Ein vorhandenes
  Ergebnis wird nie überschrieben: kein `write`, kein `delete`. Nach `outcome_unknown` denselben
  Aufruf mit demselben `request_id` noch einmal schicken, nicht mit einem neuen Namen.
- **Nur die Ergebnisdatei.** SARIF, `review-input.json` und `review-input.md` bleiben im
  Arbeitsverzeichnis.
- **Kein Geheimnis im Klartext.** Gibt die Ergebnisdatei ein Geheimnis im Klartext wieder,
  etwa ein Token aus einer Meldung, wird sie nicht abgelegt; der Abschluss sagt das.
- **Danach** ist das Dokument in Kephalaion das Ergebnis; die Fassung im Arbeitsverzeichnis
  ist nur noch Zwischenstand. `/k-review` ändert ein abgelegtes Ergebnis nicht. Abgehakt wird
  dort vom Nutzer oder von einer Sitzung auf seinen Wunsch, mit Kephalaion `write` und
  `base_revision` aus dem letzten `read`.

Ist Kephalaion nicht erreichbar (`kephalaion.ok` nicht `true` oder `create` meldet
`kephalaion_unavailable`), endet der Lauf mit der Ergebnisdatei nur im Arbeitsverzeichnis;
dasselbe gilt für jeden anderen Fehler von `create`. Ausgewichen wird nicht auf einen anderen
Ort (Fehlerfälle unten).

## Schritt 5 — Interaktiv

Generischer Ablauf für jedes interaktive Rezept:

1. **Suchen** nach den Kriterien des Rezepts; die Kandidatenliste bleibt intern.
2. **Übersicht** zeigen — kompakt, je Fund Pfad, Zeile und ein Satz. Bei leerer Liste
   ehrlich sagen „nichts gefunden“ — das ist ein gültiges Ergebnis.
3. **Rückfragen bündeln** — lassen sich Gründe oder Bewertung nicht ohne den Nutzer
   klären: **alle** Fragen in **einer** Nachricht sammeln, nicht Ping-Pong.
4. **Stelle für Stelle:**
   - Stelle mit Kontext zeigen.
   - Den Vorschlag machen, den das Rezept vorsieht (Kommentar, Fix, Notiz).
   - **Auf Freigabe warten.** Optionen: `Okay` / `Änder so: …` / `Skip`.
   - Nur nach `Okay` (oder nach Anpassung und erneutem `Okay`) tatsächlich ändern.
   - Änderung zeigen, dann die nächste Stelle.
5. **Hypothesen** — kann der Nutzer eine Rückfrage nicht beantworten: die beste Hypothese
   formulieren und **im Vorschlag** als Hypothese markieren, nicht im einzufügenden Text.
   Nach Bestätigung ohne den Hinweis einfügen.

Harte Regeln, immer:

- **Nie mehrere Stellen gleichzeitig ändern.** Je Stelle: vorschlagen → Freigabe →
  einfügen → nächste.
- **Kein Refactoring und keine Formatierung nebenbei.**
- **Keine Vermutungen als Fakten formulieren.** Falsche Begründungen sind schlimmer als gar
  keine.
- **Nur der Scope des Rezepts**, nichts anderes anfassen.

## Schritt 6 — Abschluss

**Bericht:**

- zuerst den `keph:`-Verweis des abgelegten Ergebnisses, etwa
  `keph:/results/review/2026-11-27-tech.md`, als das Ergebnis des Laufs; ist nichts abgelegt,
  stattdessen ausdrücklich, dass nichts in Kephalaion liegt, und warum (Code und Grund),
  dazu den Pfad `WORK_DIR/review-triage.md` als einzigen Ort des Ergebnisses,
- die Zahl der Bündel je Priorität `P1`/`P2`/`P3` und die Zahl der Funde im Restabschnitt,
- die Zahl der Funde, die beim Prüfen außerhalb von `audit.scope.paths` verworfen wurden,
  und eine Kürzung durch den Mengendeckel des Rezepts,
- dass keine bekannten Entscheidungen berücksichtigt wurden,
- das Arbeitsverzeichnis `WORK_DIR` als Ort von Fundliste und Beleg; es ist temporär und
  übersteht unter `/tmp/` keinen Neustart.
- Folge-Command: keiner. `/k-remediation` liest diese Ergebnisdatei noch nicht.

**Interaktiv:** kompakt N geprüft / M übernommen / K übersprungen / L nicht adressiert.
Sind nicht adressierte Funde übrig, schlag vor, sie über `/k-task-create` als Task zu
übergeben; nur nach Bestätigung. Sind keine übrig oder lehnt der Nutzer ab, sag
ausdrücklich, dass kein Folge-Command ansteht.

## Fehlerfälle

- **Rezept nicht gefunden:** verfügbare Reviews zeigen und wählen lassen (Schritt 2).
- **Mehrdeutiger Name:** alle Kandidaten zeigen, den genauen Namen erfragen.
- **Kein k-playbook-Projekt:** das Briefing scheitert; abbrechen und `/k-gui` empfehlen.
- **Arbeitsverzeichnis nicht anlegbar** (`workdir_failed`, `invalid_input`): Code und Grund
  nennen und abbrechen; keinen anderen Pfad raten.
- **Fundliste ungültig oder Merge gescheitert:** wie in den Bausteinen beschrieben
  abbrechen; eine Ergebnisdatei ohne Beleg entsteht nicht.
- **Kephalaion nicht erreichbar** (`kephalaion.ok` nicht `true` oder `create` meldet
  `kephalaion_unavailable`): Das Ergebnis bleibt nur im Arbeitsverzeichnis; der Abschluss
  sagt ausdrücklich, dass nichts in Kephalaion liegt, und warum. Kein anderer Ablageort.
- **Jeder andere Fehler von `create`** (etwa fehlendes Schreibrecht oder ein Fehler des
  Hubs): Code und Grund im Abschluss nennen; das Ergebnis bleibt nur im Arbeitsverzeichnis,
  kein Ausweichen.
