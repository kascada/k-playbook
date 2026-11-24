# k-enforcement

## Vorübergehend nicht verfügbar

Dieser Command wird neu aufgebaut, seit k-playbook keine Checks mehr ausliefert, und steht
bis dahin nicht zur Verfügung. Führe nichts von dem aus, was darunter steht — nichts lesen,
nichts schreiben, nichts anbieten. Sag dem Nutzer in einem Satz, dass `/k-enforcement`
gerade neu aufgebaut wird und erst danach wieder geht. Den Doku-Nachzug leistet bis dahin
jede Sitzung selbst, nach der Anweisung „Doku nachziehen“ im Briefing.

Der Text darunter bleibt als Vorlage für den Neuaufbau stehen.

Prüft die laufende Änderung und berichtet den Stand des Doku-Nachzugs. Das ist der ausdrückliche Prüfbericht. Den Doku-Nachzug selbst
leistet jede Sitzung schon vor dem Abschluss einer Änderung (Anweisung „Doku nachziehen“ im
Briefing, `k-playbook:/instructions/always/220-regeln-anwenden.md`); dieser Command macht die
Prüfung sichtbar und liefert sie auf Abruf, etwa aus `/k-danke`.

Produces:

- Einen Bericht. Gelesen wird der Arbeitsbaum, geändert wird nichts.
- Nur nach Freigabe durch den Nutzer: einen Doku-Auftrag in `keph:/queue/docs/`
  (`k-playbook:/instructions/on-demand/doku-auftraege.md`), wenn der Bericht für die erzeugte
  Doku „nachzuziehen“ ergibt. Korrekturen am Code oder an der Doku nur, wenn der Nutzer sie in
  derselben Sitzung ausdrücklich verlangt.

## Schritt 1 — Briefing und Prüfumfang

include: _shared/erster-schritt.md

Aus dem Briefing brauchst du:

- `session` — die Kennung dieser Sitzung (Klartext: Zeile `Sitzung:`).
- `project.dir`, `project.repoRoot`.
- `kephalaion` — `ok`, `address`, `message` — für den Doku-Nachzug.

**Prüfumfang.** Geprüft werden die geänderten Dateien des Arbeitsbaums gegen `HEAD`, samt
unverfolgten. Du bestimmst sie einmal und hältst sie in einer Datei fest:

```bash
ROOT="<project.repoRoot>"
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
`ROOT`. Gelöschte Dateien gehören dazu; sie zählen für den Doku-Nachzug
(etwa eine entfernte Funktion, die die Doku noch nennt).

- `$ARGUMENTS` nennt ein Verzeichnis oder eine Datei innerhalb von `ROOT`: Dann gilt nur der
  Teil der Menge darunter; schreibe die gekürzte Liste in dieselbe Datei. Liegt das Argument außerhalb von `ROOT`, abbrechen und es sagen.
- Ist `ROOT` kein Git-Repo, oder hat es noch keinen Commit (`git diff HEAD` scheitert), gibt
  es keine Menge aus dem Arbeitsbaum: Frag, welche Dateien geprüft werden sollen, und schreib
  die Antwort in die Datei.
- Ist die Menge leer, sag „kein Diff“; der Doku-Nachzug wird dann nicht geprüft, und der
  Bericht nennt nur den leeren Prüfumfang.

Command-specific policy:

- Ohne Kephalaion (`kephalaion.ok` nicht `true`) ist die erzeugte Doku nicht
  erreichbar: Der Doku-Nachzug wird nicht geprüft und nicht eingereiht, der Bericht nennt ihn
  als offen mit `kephalaion.message`. Es gibt keinen Rückfall auf die Platte.

```text
Enforcement-Check
─────────────────────────────
Prüfumfang:   <N> Dateien unter <ROOT> | kein Diff | <ARGUMENTS>
```

## Schritt 2 — Doku-Nachzug prüfen

Immer, wenn der Prüfumfang Code enthält. Maßstab ist der Absatz zum Doku-Nachzug in
`k-playbook:/instructions/always/220-regeln-anwenden.md`; lies ihn mit `k_playbook_doc_read`
(ersatzweise `k-playbook read <verweis>`), wenn er dir nicht schon aus dem Briefing vorliegt.

- **Handgeschriebene Doku** — Handdoku, `README.md`, die projekteigenen Regeln (Instructions und Ähnliches wie Skills), Architektur-, Setup- und
  API-Beschreibungen: Sind sie im selben Arbeitsgang angepasst, oder ist begründet, warum
  keine Änderung nötig ist?
- **Erzeugte Doku** — `keph:/docs/generated/code/` (Erzeuger `code`) und
  `keph:/docs/generated/libs/` (Erzeuger `tools`). Sie wird nicht von Hand nachgezogen und
  nicht in diesem Command erzeugt. Betrifft die Änderung einen Erzeuger (Code im Scope von
  `code`, Abhängigkeiten oder Werkzeuge für `tools`), lautet der Ausgang „nachzuziehen“.
  Ob schon ein Auftrag dieser Sitzung deckt („Kein Doppel“) oder noch keine erzeugte Doku
  liegt, entscheidet das Programm beim Einreihen unten, nicht dieser Text.

Ein veraltetes Dokument unter `libs/`, `versions/` oder `manual/` meldest du nicht als Folge
einer Codeänderung; die `README.md` eines Erzeuger-Verzeichnisses schreibt `publish`.

Ausgang je Teil, einer davon:

- **angepasst** — die Handdoku wurde mit dem Code geändert.
- **eingereiht** — für die erzeugte Doku liegt ein offener roher Auftrag dieser Sitzung vor;
  bei `code` decken seine `pfade` die Änderung.
- **nachzuziehen** — die erzeugte Doku veraltet, es gibt noch keinen Auftrag, oder dem
  Auftrag für `code` fehlen Pfade der Änderung.
- **nicht nötig** — mit Begründung in einem Satz. Reine Formatierung, Tests für bestehendes
  Verhalten und interne Umbauten ohne geänderte Semantik, die keine Doku beschreibt, brauchen
  in der Regel keine Doku-Änderung.
- **fehlt** — die wahrscheinlich betroffene Handdoku, mit Pfad oder Thema.
- **unklar** — Frage an den Nutzer: anpassen, anlegen oder bewusst auslassen.

Keine Codeänderung geht ohne einen dieser Ausgänge durch. Die Datei `CHANGED_FILES` löschst
du danach.

## Schritt 3 — Bericht

```text
Enforcement
─────────────────────────────
Geprüfter Scope: <N> Dateien | kein Diff

Docs-Sync:       Handdoku angepasst | nicht nötig (<Grund>) | fehlt (<Pfad/Thema>) | unklar
                 code:  eingereiht (<verweis>) | nachzuziehen | noch keine erzeugte Doku | nicht betroffen
                 tools: eingereiht (<verweis>) | nachzuziehen | noch keine erzeugte Doku | nicht betroffen
```

Ist etwas `unklar` oder `fehlt`, nenne die konkrete nächste Aktion
und frag, bevor du etwas änderst.

**Doku-Auftrag nach Freigabe.** Steht bei `code` oder `tools` „nachzuziehen“, frag, ob der
Nachzug eingereiht werden soll. Nach Ja, je Erzeuger: `k_playbook_docs_order_add` mit
`projectDir` = `project.dir`, `erzeuger` (`code` bzw. `tools`), `pfade` aus dem
Prüfumfang, so fein wie möglich, `anlass` (ein Satz) und `sitzung` = `session` (ersatzweise
`k-playbook docs order add <code|tools> --pfad <pfad>… --anlass "<satz>" --sitzung <session>
--json`). „Kein Doppel“ und „Wann nicht eingereiht wird“ rechnet das Programm nach
`k-playbook:/instructions/on-demand/doku-auftraege.md`; die Antwort sagt `status`:

- `created` oder `extended` — eingereiht bzw. der vorhandene Auftrag dieser Sitzung ergänzt:
  im Bericht „eingereiht“, mit dem Verweis `ref`.
- `covered` — ein offener Auftrag deckt die Änderung schon: „eingereiht“, mit `ref`.
- `skipped` — nichts eingereiht: `no_generated_docs` heißt „noch keine erzeugte Doku“,
  `kephalaion_unavailable` heißt, der Nachzug bleibt offen; der Abschluss nennt es.

Bei Nein bleibt es bei „nachzuziehen“; der Abschluss nennt es als offen. `/k-docs-tools`
sofort laufen zu lassen bleibt erlaubt. Für `code` gibt es den sofortigen Lauf nur als vollen
Lauf: `/k-docs code` reiht ihn ein, `/k-docs-code` arbeitet ihn ab.

Folge-Command: keiner. Der Bericht und die genannte nächste Aktion sind das Ergebnis;
korrigiert wird nur nach Freigabe in derselben Sitzung. Eingereihte Aufträge arbeitet
`/k-docs-code` bzw. `/k-docs-tools` ab, `/k-docs` zeigt sie.

## Fehlerfälle

- `k_playbook_docs_order_add` ist unbekannt (`invalid_input` für das Werkzeug) oder `k-playbook
  docs` meldet `unbekannter Unterbefehl` → die Installation ist älter als dieser Command:
  melden, `/k-upgrade` nennen, nichts von Hand anlegen oder ersetzen.
- `skipped` mit `kephalaion_unavailable` beim Einreihen → nicht eingereiht; der Bericht nennt
  den Nachzug als offen. `outcome_unknown` → nicht wiederholen, mit `k_playbook_doc_list`
  (`kind: docs`, `name` oder `number` des Auftrags) nachsehen.

## Anti-Muster (nicht tun)

- **Erzeugte Doku von Hand nachziehen** oder den Erzeuger ungefragt starten. Der Normalfall
  ist der Auftrag, und der kommt nur nach Freigabe.
- **Einen Auftrag von Hand anlegen oder ergänzen.** Das tut `k_playbook_docs_order_add`; es
  legt keinen zweiten für dieselbe Sitzung an.
