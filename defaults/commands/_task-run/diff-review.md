---
title: diff-review
description: Ablauf des Review-Sub-Agenten von /k-task-run — den Diff eines Tasks gegen seinen Snapshot bilden, die Abschnitte mit k_playbook_doc_append an den Task anhängen, nach dem übergebenen Rezept prüfen und das Ergebnis samt Revision in festem Format zurückgeben.
---

# diff-review

Dieses Modul ist kein Review-Katalog-Rezept. Es beschreibt den Ablauf des
Review-Sub-Agenten von `/k-task-run`: den Diff eines Tasks gegen den Snapshot des Tasks
bilden, an das Task-Dokument anhängen und nach dem übergebenen Rezept prüfen. Eingebunden
wird es nur von `/k-task-run`, in 5f; es liegt deshalb im Modulverzeichnis
`commands/_task-run/` dieses Commands. Der Hauptkontext von `/k-task-run` liest das Modul
nicht; er gibt dem Sub-Agenten nur den Verweis `k-playbook:/commands/_task-run/diff-review.md`,
und der liest ihn mit `k_playbook_doc_read`.

Was geprüft wird und wie ein Befund eingestuft wird, steht allein im Rezept — in seinem
Abschnitt für den Diff eines Tasks und in den Prüfkriterien, die dieser Abschnitt anwendet.
Dieses Modul wiederholt davon nichts. Es hält nur den Ablauf, die Form der Abschnitte im
Task-Dokument und das Rückgabeformat.

Überschriftenzeilen und Rückgabeformat sind der Vertrag zwischen diesem Modul und
`/k-task-run`: Der Command zählt die Überschriftenzeilen, um zu prüfen, ob ein Lauf seinen
Abschnitt geschrieben hat, und wertet die Rückgabe aus. Die Überschriftenzeilen stehen auch
in der Kennzeilen-Tabelle von `k-playbook:/instructions/on-demand/tasks.md`. Wer eines davon
ändert, zieht `k-playbook:/commands/k-task-run.md` und diese Tabelle im selben Zug nach.

## Eingaben

Der Hauptkontext übergibt:

- den `keph:`-Verweis des Task-Dokuments und seine aktuelle Revision,
- `projectDir` für die k-playbook-Werkzeuge: `context.project.dir` des Hauptkontexts,
- die Ausführungswurzel — sie gilt nur für `git`. Liegt sie woanders als `projectDir`
  (`Target repo` im `## Ausführungskontext`), bleibt `projectDir` trotzdem das Projekt.
- die Kennung des Snapshots dieses Tasks — ein Tree-Objekt in der Ausführungswurzel,
- den Umfang: `ab Task-Beginn` oder `nur seit Fortsetzung`,
- die Zeilen „Validierung“ aus der letzten Rückgabe von Ausführung oder Behebung, oder den
  Vermerk, dass es keine gibt,
- entweder den Verweis des wirksamen Rezepts (etwa `k-playbook:/reviews/review-code.md`) —
  dann laufen Teil 1 und Teil 2 — oder die Anweisung „nur Teil 1“.

## Grundsätze

- **Keine Rückfragen.** Der Sub-Agent fragt den Nutzer nicht. Offenes kommt nur über die
  Rückgabe zurück: als Befund oder, lässt sich das Review nicht abschließen, als Abbruch
  (Abschnitt „Abbruch“).
- **Nur anhängen, mit Revision.** Der Sub-Agent hängt seine Abschnitte mit
  `k_playbook_doc_append` an das Task-Dokument an (`ref` = der Verweis, `content` = der
  Abschnitt, `since_revision` = die übergebene Revision bzw. die Revision seines letzten
  eigenen Anhängens; `projectDir` = das übergebene `projectDir`, nicht die
  Ausführungswurzel). Ersatzweise ohne MCP, im Verzeichnis `projectDir`:
  `k-playbook doc append <ref> --since <n> --json <<'EOF'` mit der Endmarke in
  Anführungszeichen; `--json` liefert `revision` und bei einem Fehler den Code
  (`foreign_change`, `outcome_unknown`). Er ändert keine bestehende Zeile — keine
  Fortschrittszeile, nicht den Kopf von `## Ausführung` und keinen Abschnitt eines früheren
  Laufs. Kennzeile, Snapshot-Zeile, `**Zusammenfassung:**` und `**Status nach Review:**`
  schreibt allein der Hauptkontext.
- **`foreign_change` hält an.** Antwortet `k_playbook_doc_append` mit `foreign_change`, hat
  seit der übergebenen Revision jemand anderes in den Task geschrieben. Der Sub-Agent schreibt
  nichts weiter und beendet den Lauf mit der Zeile `Foreign-Change: …` aus dem Abschnitt
  „Rückgabe“.
- **Vor jedem Anhängen zählen.** Vor dem Anhängen eines Abschnitts zählt der Sub-Agent mit
  `k_playbook_doc_grep` und `count_only` dessen Überschriftenzeile
  (`^### Diff: Geänderte Dateien$` bzw. `^### Review: Befunde$`) und merkt sich die Anzahl, nach
  `k-playbook:/instructions/on-demand/tasks.md`, „Zählen vor dem Anhängen“. Nach
  `outcome_unknown` hängt er nicht blind noch einmal an, sondern zählt erneut: Ist die Zahl
  gestiegen, ist der Abschnitt angekommen, und die Revision aus dieser Antwort gilt; sonst
  folgt er dem festen Weg aus „`outcome_unknown` je Operation“ dort. Eine ältere Revision
  direkt nach dem eigenen Anhängen ist der Replica-Nachlauf aus demselben Abschnitt, kein
  fremder Schreiber.
- **Nichts beheben.** Das Review ändert keinen Code und keine andere Datei; es schreibt nur
  in das Task-Dokument.
- **Feste Reihenfolge.** Teil 1 läuft immer, Teil 2 nur nach Teil 1 und nur, wenn ein
  Rezept übergeben wurde. Jeder Lauf hängt seine Abschnitte neu an; die eines früheren Laufs
  bleiben stehen.

## Überschriftenzeilen

Jede steht in einem Lauf genau einmal, am Zeilenanfang, und sonst nirgends in dem, was der
Sub-Agent schreibt:

| Zeile | Teil | Zählt `/k-task-run` |
|---|---|---|
| `### Diff: Geänderte Dateien` | 1 | ja — Diff-Abschnitte vorhanden |
| `### Diff: Code-Änderungen` | 1 | nein |
| `### Review: Befunde` | 2 | ja — Review-Abschnitt vorhanden |

Sie unterscheiden sich bewusst von den Überschriften des `## Review-Log` aus
`/k-task-refine`, wo etwa `### Geänderte Dateien` steht.

## Teil 1 — Diff-Abschnitte

### Stand nach dem Task festhalten

Der Snapshot ist ein Tree-Objekt, das der Hauptkontext beim ersten Start des Tasks über
einen temporären Index angelegt hat; er enthält nicht committete und ungetrackte Dateien.
Den Stand nach dem Task hält der Sub-Agent auf dieselbe Weise fest, damit beide Seiten
vergleichbar sind und neu angelegte Dateien im Diff erscheinen. Der echte Index und der
Arbeitsbaum bleiben unberührt. Die Ausgabe ist allein die Kennung des Stands:

```bash
if command -v git >/dev/null 2>&1; then
  (
    cd "<Ausführungswurzel>" || exit 1
    git cat-file -e "<Snapshot>^{tree}" || exit 1
    now_dir="$(mktemp -d)" || exit 1
    cp "$(git rev-parse --git-path index)" "$now_dir/index" 2>/dev/null || true
    GIT_INDEX_FILE="$now_dir/index" git add -A &&
      now="$(GIT_INDEX_FILE="$now_dir/index" git write-tree)" &&
      printf '%s\n' "$now"
    status=$?
    rm -rf "$now_dir"
    exit "$status"
  )
else
  echo "git fehlt: kein Diff" >&2
fi
```

Lässt sich der Snapshot nicht auflösen, fehlt `git` oder scheitert ein Aufruf, ist Teil 1
fehlgeschlagen.

### Diff bilden

Verglichen werden die beiden Trees. Das Task-Dokument liegt in der Collection, nicht im
Arbeitsbaum; es gibt nichts auszunehmen.

```bash
if command -v git >/dev/null 2>&1; then
  git -C "<Ausführungswurzel>" diff --stat "<Snapshot>" "<Stand>"
  git -C "<Ausführungswurzel>" diff "<Snapshot>" "<Stand>"
fi
```

Nicht erfasst sind Dateien, die `.gitignore` ausschließt; das gilt für Snapshot und Stand
gleichermaßen.

### Abschnitte schreiben

```text
### Diff: Geänderte Dateien

**Umfang:** <ab Task-Beginn | nur seit Fortsetzung>  
**Snapshot:** `<Kennung>`

<Ausgabe von git diff --stat in einem Codeblock, oder „Keine Änderungen seit dem Snapshot.“>

### Diff: Code-Änderungen

<Ausschnitte nach der Kürzungsregel, oder „Keine Änderungen seit dem Snapshot.“>
```

Kürzungsregel für „Code-Änderungen“:

- Nur die wesentlichen Hunks; generierte Dateien, Lockfiles und Binärdateien entfallen.
- Ist der Diff länger als etwa 100 Zeilen, wird Nebensächliches in Prosa zusammengefasst,
  und nur die wichtigsten Hunks stehen wörtlich.
- Hunks stehen wörtlich im unified-diff-Format, in einem Codeblock mit der Sprachkennung
  `diff` und einem Zaun aus mindestens vier Backticks, länger als jede Backtick-Folge im
  Ausschnitt. So beginnt keine Diff-Zeile mit einer Zeile, die `/k-task-run` am
  Zeilenanfang sucht, und kein Codeblock aus dem Diff schließt den Zaun vorzeitig.
- Prosa ohne Überschriften.

Läuft nur Teil 1, ist hier Schluss. Die Rückgabe sind genau zwei Zeilen:

```text
Diff: geschrieben | fehlgeschlagen — <Grund>
Revision: <n>
```

Schlägt Teil 1 in einem Lauf mit Teil 2 fehl, läuft Teil 2 nicht; es gilt der Abschnitt
„Abbruch“.

## Teil 2 — Review

### Rezept anwenden

Lies das Rezept am übergebenen Verweis mit `k_playbook_doc_read`. Seinen Abschnitt für den
Diff eines Tasks erkennst du an genau dieser Überschriftenzeile:

```text
## Ergebnisform für den Diff eines Tasks
```

Wende diesen Abschnitt an: Gegenstand, Maßstab, Einstufung und den Beleg der
aufgabengebundenen Kriterien nimmst du von dort, die Prüfkriterien und Rule-IDs aus dem
übrigen Rezept, auf das der Abschnitt verweist.

Fehlt dem Rezept — etwa einem projekteigenen Overlay — dieser Abschnitt oder darin die
Einstufung, stufst du nicht selbst ein und greifst auf keinen anderen Review-Weg zurück: Es
gilt der Abschnitt „Abbruch“.

- **Gegenstand** ist der vollständige Diff aus Teil 1, nicht der gekürzte Ausschnitt im
  Task-Dokument.
- **Maßstab** ist die Task-Beschreibung, gelesen mit `k_playbook_doc_read` und geschnitten
  nach `k-playbook:/instructions/on-demand/tasks.md`, „Rumpf und Lauf-Abschnitte“. Was
  `/k-task-run` und dieses Modul in das Dokument geschrieben haben — Fortschrittszeilen,
  `## Ausführung` und alles darunter —, gehört nicht dazu.
- **Build und Tests.** Beleg sind die übergebenen Zeilen „Validierung“. Fehlen sie oder
  decken sie die Validierung nicht ab, die der Task vorschreibt, führst du die dort
  vorgeschriebenen Prüfbefehle in der Ausführungswurzel selbst aus. Lässt sich ein solcher
  Befehl nicht ausführen, steht das unter „Beleg für Build und Tests“; der Lauf bricht
  deshalb nicht ab.

### Abschnitt schreiben

```text
### Review: Befunde

**Umfang:** <ab Task-Beginn | nur seit Fortsetzung>  
**Rezept:** `<Verweis des Rezepts>`  
**Beleg für Build und Tests:** <übergeben | selbst ausgeführt: Befehl → Ergebnis | keiner — Grund>  
**Ergebnis:** <n> kritisch, <n> wichtig, <n> Hinweis

| Einstufung | Ort | Grundlage | Befund |
|---|---|---|---|
| <Einstufung aus dem Rezept> | `<datei:zeile>` oder `<Prüfbefehl>` | <Rule-ID oder aufgabengebundenes Kriterium> | <ein Satz, mit dem Beleg, den das Rezept verlangt> |
```

- Befunde in der Reihenfolge kritisch, wichtig, Hinweis; ohne Befunde statt der Tabelle
  „Keine Befunde.“
- Ein Befund zu Build oder Tests trägt statt `datei:zeile` den Prüfbefehl als Ort.
- Die Zahlen unter „Ergebnis“ stimmen mit der Tabelle überein.

### Rückgabe

Nach dem Abschnitt gibt der Sub-Agent genau das zurück, ohne Diff, Dateiinhalte oder Logs:

```text
Review: <n> kritisch, <n> wichtig, <n> Hinweis
Umfang: ab Task-Beginn | nur seit Fortsetzung
Kritisch:
  - <datei:zeile oder Prüfbefehl> — <ein Satz>
Revision: <n>
```

Ohne kritischen Befund lautet die dritte Zeile `Kritisch: keine`. „Umfang“ wiederholt den
übergebenen Umfang.

**`Revision: <n>`** steht in jeder Rückgabe dieses Moduls — nach Teil 1 allein, nach Teil 2
und beim Abbruch — und nennt die Revision des letzten eigenen Anhängens; wurde nichts
angehängt, die übergebene. Damit setzt der Hauptkontext sein nächstes `since_revision`.
Hat ein `k_playbook_doc_append` mit `foreign_change` geantwortet, steht statt `Revision: <n>`
die Zeile

```text
Foreign-Change: <revision> <updatedBy> <updatedAt>
```

mit den Angaben aus der Antwort; der Sub-Agent hat dann nichts weiter geschrieben, und der
Hauptkontext fragt den Nutzer, wer weitermacht.

## Abbruch

Lässt sich das Review nicht abschließen — Teil 1 ist fehlgeschlagen, das Rezept ist am
Verweis nicht lesbar, ihm fehlt der Abschnitt oder darin die Einstufung —, schreibt der
Sub-Agent keinen Abschnitt `### Review: Befunde` und gibt statt der Rückgabe oben genau
diese Zeilen zurück:

```text
Abbruch: <Grund>
Revision: <n>
```

`/k-task-run` wertet jede Rückgabe, die nicht dem Format aus „Rückgabe“ folgt, und jeden
Lauf ohne neuen Review-Abschnitt wie einen kritischen Befund (fail-closed). Ein Ersatz wie
`Review: 0 kritisch, …` für ein nicht abgeschlossenes Review ist deshalb ausgeschlossen.
