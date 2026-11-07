# k-enforcement

Prüft die laufende Änderung gegen die Regeln des Projekts und berichtet je Regel, ob sie
eingehalten ist — dazu das Ergebnis von `k-check --mode changed` und den Stand des
Doku-Nachzugs. Das ist der ausdrückliche Prüfbericht. Die Regeln selbst wendet jede Sitzung
schon vor dem Abschluss einer Änderung an (Anweisung „Regeln anwenden“ im Briefing); dieser
Command macht die Prüfung sichtbar und liefert sie auf Abruf, etwa aus `/k-danke`.

Produces:

- Einen Bericht. Gelesen wird der Arbeitsbaum, geändert wird nichts.
- Nur nach Freigabe durch den Nutzer: einen Doku-Auftrag in `keph:/queue/docs/`
  (`k-playbook:/instructions/on-demand/doku-auftraege.md`), wenn der Bericht für die erzeugte
  Doku „nachzuziehen“ ergibt. Korrekturen am Code oder an der Doku nur, wenn der Nutzer sie in
  derselben Sitzung ausdrücklich verlangt.

## Schritt 1 — Briefing und Prüfumfang

include: _shared/erster-schritt.md

Aus dem Briefing brauchst du:

- `documents.rules` — die wirksamen Regeln, schon verrechnet: mitgeliefert, lokal ersetzt
  oder abgeschaltet, mit `origin` je Eintrag. Verzeichnisse listest du nicht selbst.
- `session` — die Kennung dieser Sitzung (Klartext: Zeile `Sitzung:`).
- `context.project.dir`, `context.project.repoRoot`, `context.playbook.dir`.
- `context.kephalaion` — `ok`, `address`, `message` — für den Doku-Nachzug.

**Prüfumfang.** Geprüft werden die geänderten Dateien des Arbeitsbaums gegen `HEAD`, samt
unverfolgten. Regelprüfung und `k-check` nehmen dieselbe Menge; deshalb bestimmst du sie
einmal und gibst sie `k-check` als Datei mit:

```bash
ROOT="<context.project.repoRoot>"
if command -v git >/dev/null 2>&1; then
  CHANGED_FILES="$(mktemp)"
  { git -C "$ROOT" diff --name-only HEAD; git -C "$ROOT" ls-files --others --exclude-standard; } \
    | sort -u > "$CHANGED_FILES"
  echo "$CHANGED_FILES"; cat "$CHANGED_FILES"
else
  echo "git fehlt — kein Prüfumfang aus dem Arbeitsbaum"
fi
```

Die erste Zeile der Ausgabe ist die Datei mit der Menge; merke sie dir als `CHANGED_FILES` —
jeder Shell-Aufruf beginnt neu, die Variable überlebt ihn nicht. Die Pfade sind relativ zu
`ROOT`. Gelöschte Dateien gehören dazu; sie zählen für die Regeln
(etwa eine entfernte Funktion, die die Doku noch nennt).

- `$ARGUMENTS` nennt ein Verzeichnis oder eine Datei innerhalb von `ROOT`: Dann gilt nur der
  Teil der Menge darunter, für die Regeln und für `k-check`; schreibe die gekürzte Liste in
  dieselbe Datei. Liegt das Argument außerhalb von `ROOT`, abbrechen und es sagen.
- Ist `ROOT` kein Git-Repo, oder hat es noch keinen Commit (`git diff HEAD` scheitert), gibt
  es keine Menge aus dem Arbeitsbaum: Frag, welche Dateien geprüft werden sollen, und schreib
  die Antwort in die Datei. Will der Nutzer nur die Regeln sehen, fasse sie zusammen und
  ende.
- Ist die Menge leer, sag „kein Diff“; die Regeln werden dann nicht geprüft, `k-check` läuft
  nicht, und der Bericht nennt nur die geladenen Regeln.

Command-specific policy:

- Eine leere Regelmenge (`documents.rules` ohne aktiven Eintrag) ist eine Entscheidung des
  Projekts. Sag das und prüfe keine Regeln; es gibt keinen Rückfall auf die mitgelieferten.
- Ohne Kephalaion (`context.kephalaion.ok` nicht `true`) ist die erzeugte Doku nicht
  erreichbar: Der Doku-Nachzug wird nicht geprüft und nicht eingereiht, der Bericht nennt ihn
  als offen mit `context.kephalaion.message`. Es gibt keinen Rückfall auf die Platte.

## Schritt 2 — Regeln laden

Lies jede Regel aus `documents.rules`, deren `origin` nicht `disabled` ist, mit
`k_playbook_doc_read` und ihrem Verweis als `ref` (ersatzweise `k-playbook read <verweis>`),
in der Reihenfolge des Briefings. Einträge mit `disabled` überspringst du und zählst sie.

```text
Enforcement-Check
─────────────────────────────
Prüfumfang:   <N> Dateien unter <ROOT> | kein Diff | <ARGUMENTS>
Regeln:       <N> aktiv  (<A> dist, <B> local, <C> override)
Abgeschaltet: <D> | —
```

## Schritt 3 — Regeln prüfen

Für jede geladene Regel:

1. Entscheide, ob sie den Prüfumfang betrifft. Eine Regel, die keine der Dateien berührt, ist
   „nicht betroffen“.
2. Ist sie betroffen, prüfe, ob der Stand der Dateien sie erfüllt.
3. Ist es unklar, stell eine kurze Frage, statt zu raten.

Die Regeln gelten für neue und geänderte Arbeit, nicht rückwirkend für den ganzen Bestand,
außer eine Regel sagt das ausdrücklich.

## Schritt 4 — Doku-Nachzug prüfen

Immer, wenn der Prüfumfang Code enthält. Maßstab ist `k-playbook:/rules/docs-sync.md` aus
Schritt 2.

- **Handgeschriebene Doku** — Handdoku, `README.md`, `AGENTS.md`, Architektur-, Setup- und
  API-Beschreibungen: Sind sie im selben Arbeitsgang angepasst, oder ist begründet, warum
  keine Änderung nötig ist?
- **Erzeugte Doku** — `keph:/docs/generated/code/` (Erzeuger `code`) und
  `keph:/docs/generated/libs/` (Erzeuger `tools`). Sie wird nicht von Hand nachgezogen und
  nicht in diesem Command erzeugt. Je Erzeuger, den die Änderung betrifft:
  1. Gibt es dort noch keine Dokumente (Kephalaion `list` mit `collection` =
     `context.kephalaion.address`, `path` = `docs/generated/code` bzw. `docs/generated/libs`),
     ist nichts nachzuziehen: „noch keine erzeugte Doku“.
  2. Sonst suchst du mit `k_playbook_doc_grep`, `kind: docs`, Muster
     `^sitzung: <session>$`, einen offenen Auftrag dieser Sitzung, und prüfst für die Treffer
     `^erzeuger: code$` bzw. `^erzeuger: tools$`. Gibt es ihn: „eingereiht“, mit Verweis.
  3. Sonst: „nachzuziehen“. Das Einreihen bietet Schritt 6 an.

Ein veraltetes Dokument unter `libs/`, `versions/` oder `manual/` meldest du nicht als Folge
einer Codeänderung; die `README.md` eines Erzeuger-Verzeichnisses schreibt `publish`.

Ausgang je Teil, einer davon:

- **angepasst** — die Handdoku wurde mit dem Code geändert.
- **eingereiht** — für die erzeugte Doku liegt ein offener Auftrag dieser Sitzung vor.
- **nachzuziehen** — die erzeugte Doku veraltet, es gibt noch keinen Auftrag.
- **nicht nötig** — mit Begründung in einem Satz.
- **fehlt** — die wahrscheinlich betroffene Handdoku, mit Pfad oder Thema.
- **unklar** — Frage an den Nutzer: anpassen, anlegen oder bewusst auslassen.

Keine Codeänderung geht ohne einen dieser Ausgänge durch.

## Schritt 5 — k-check ausführen

Führe die Checks über dieselbe Dateimenge aus, mit dem Programm aus der Installation. Die
Pfade in `CHANGED_FILES` sind relativ zu `ROOT`; deshalb bekommt `k-check` dieselbe Wurzel
ausdrücklich als `--target-root`, die Konfiguration liest es weiter aus dem Projekt:

```bash
"<context.playbook.dir>/bin/k-check" --mode changed --config-root "<context.project.dir>" --target-root "<context.project.repoRoot>" --files-from "<CHANGED_FILES>"
```

`k-check` findet die wirksamen Checks selbst. Berichte je Check die Zeile seiner Ausgabe
(`OK`, `FAIL`, `SKIP`, `ERROR` mit Grund) und den Exit-Code: `0` alles grün, `1` mindestens
ein Check rot, `2` ein technischer Fehler. Ein Ergebnis, das du nicht gesehen hast, meldest du
nicht. Fehlt `k-check` oder bricht es ab, nenne die Meldung; die Regelprüfung bleibt gültig.
Die Datei `CHANGED_FILES` löschst du danach.

## Schritt 6 — Bericht

```text
Enforcement
─────────────────────────────
Regeln geladen:  <N> aktiv (<A> dist, <B> local, <C> override, <D> abgeschaltet)
Geprüfter Scope: <N> Dateien | kein Diff
Betroffen:       <Regeln>

Ergebnis:
- <regel> [dist|local|override]: ok | offen | unklar | verletzt | nicht betroffen

k-check:         grün | rot | Fehler (Exit <n>) | nicht gelaufen (<Grund>)
- <check>: OK | FAIL | SKIP | ERROR — <Grund>

Docs-Sync:       Handdoku angepasst | nicht nötig (<Grund>) | fehlt (<Pfad/Thema>) | unklar
                 code:  eingereiht (<verweis>) | nachzuziehen | noch keine erzeugte Doku | nicht betroffen
                 tools: eingereiht (<verweis>) | nachzuziehen | noch keine erzeugte Doku | nicht betroffen
```

Ist etwas `offen`, `unklar`, `verletzt`, rot oder `fehlt`, nenne die konkrete nächste Aktion
und frag, bevor du etwas änderst.

**Doku-Auftrag nach Freigabe.** Steht bei `code` oder `tools` „nachzuziehen“, frag, ob der
Nachzug eingereiht werden soll. Nach Ja, je Erzeuger:

1. `k_playbook_doc_create` mit `kind: docs` und einem Kurznamen aus Erzeuger und Anlass; die
   Antwort nennt den Verweis.
2. Gleich danach Kephalaion `write` mit `collection` = `context.kephalaion.address`, `name` =
   Verweis ohne `keph:/`, den Inhalt mit dem Kopf aus
   `k-playbook:/instructions/on-demand/doku-auftraege.md`: `erzeuger`, `pfade` aus dem
   Prüfumfang, `anlass`, `sitzung` = `session`, `entstanden` = `context.now.date`.
3. Nenne den Verweis; im Bericht steht dann „eingereiht“.

Bei Nein bleibt es bei „nachzuziehen“; der Abschluss nennt es als offen. `/k-docs-code` bzw.
`/k-docs-tools` sofort laufen zu lassen bleibt erlaubt.

Folge-Command: keiner. Der Bericht und die genannte nächste Aktion sind das Ergebnis;
korrigiert wird nur nach Freigabe in derselben Sitzung. Eingereihte Aufträge arbeitet
`/k-docs-code` bzw. `/k-docs-tools` ab, `/k-docs` zeigt sie.

## Fehlerfälle

- `documents.rules` fehlt im Briefing → das Briefing ist älter als dieser Command: neu laden
  (`k_playbook_briefing`); fehlt es weiter, `/k-upgrade` nennen.
- Eine Regel ist nicht lesbar (`not_found`) → nennen und mit den übrigen weitermachen.
- `k_playbook_doc_create` meldet `invalid_input` für die Sorte `docs` → die Installation ist
  älter als dieser Command: melden, `/k-upgrade` nennen, nichts von Hand anlegen.
- `kephalaion_unavailable` beim Einreihen → nicht eingereiht; der Bericht nennt den Nachzug
  als offen. `outcome_unknown` → nicht wiederholen, mit `k_playbook_doc_list` (`kind: docs`)
  nachsehen.

## Anti-Muster (nicht tun)

- **Die Regeln selbst zusammensuchen.** Welche gelten, sagt `documents.rules`; wer
  Verzeichnisse listet, erwischt eine abgeschaltete oder ersetzte Fassung.
- **Zwei Prüfumfänge.** Regeln und `k-check` prüfen dieselbe Dateimenge; ohne `--files-from`
  nähme `k-check` gegebenenfalls einen anderen Vergleichsstand.
- **Erzeugte Doku von Hand nachziehen** oder den Erzeuger ungefragt starten. Der Normalfall
  ist der Auftrag, und der kommt nur nach Freigabe.
- **Einen zweiten Auftrag anlegen**, wenn diese Sitzung schon einen für denselben Erzeuger
  hat.
- **Ein Check-Ergebnis berichten, das nicht gelaufen ist.**
