# Betrieb: Wo abgelegte Dateien landen

Diese Guideline legt fest, wohin eine Datei gehört, die im Lauf der Arbeit entsteht —
eine Task, ein Review-Ergebnis, eine Doku-Seite, eine Notiz, Rohmaterial. Sie ist die
eine Auskunft dazu. Ein Command, ein Skill oder eine Regel, die eine Datei ablegt,
verweist hierher, statt den Ort ein zweites Mal zu begründen.

## Zwei Grundsätze

**In `k-playbook/` wird nie geschrieben.** Das Verzeichnis ist ein Clone und wird bei
jedem Update vollständig ersetzt. Was dort abgelegt wird, ist beim nächsten Update
weg — ohne Fehlermeldung, ohne Spur. Alles, was das Projekt hervorbringt, liegt
daneben in `k-playbook-local/`.

**Pfade werden nicht geraten.** Der Ort wird aus der Ausgabe von `k-playbook context`
abgeleitet: `local.dir` für alles Projekteigene, `playbook.dir` für die Installation.
Ein fest verdrahteter Pfad geht in dem Moment kaputt, in dem ein Projekt anders heißt
oder tiefer liegt. Für Text, den ein Mensch liest, bleibt die kurze Schreibweise
`k-playbook-local/...` richtig; zum Schreiben wird der aufgelöste Pfad benutzt.

## Was wohin

Alle Pfade relativ zu `local.dir`, also `k-playbook-local/`.

| Was | Wohin | Wer legt es ab |
|---|---|---|
| Task, offen | `tasks/<NNN>-<slug>.md` | `/k-task-create`, `/k-remediation` |
| Task, erledigt | `tasks/done/<NNN>-<slug>.md` | `/k-task-run` verschiebt sie dorthin |
| Kurznotiz, unsortiert | `data/todos.json` | `/k-todo`, `k-playbook todo`, die Oberfläche |
| Review- und Audit-Lauf | `results/<YYYY-MM-DD>/` bzw. `results/<familie>/<YYYY-MM-DD>/` | `/k-review`, `/k-audit` |
| Lauf-Protokoll | `results/log.md` | `/k-review`, `/k-audit` |
| Bewusste Entscheidung zu einem Befund | `known-decisions.md` | von Hand |
| Doku aus dem Code | `docs/code/<NN>-<slug>.md` | `/k-docs-code` |
| Steckbrief zu Library oder Tool | `docs/libs/<name>.md` | `/k-docs-tools` |
| Doku aus Rohmaterial | `docs/extracted/<NN>-<slug>.md` | `/k-docs-extract` |
| Versionsinventar | `docs/versions/inventory.md` | `/k-doc-inventory`, `k-playbook inventory` |
| Handgeschriebene Doku | `docs/manual/<slug>.md` | von Hand |
| Doku-Index | `docs/README.md` | `/k-docs-index`, sonst niemand |
| Rohmaterial: Verläufe, Mitschriften, Übergaben | `material/` | von Hand |
| Notiz, Zwischenstand, Privates | `priv/` | von Hand |
| Quellen des Versionsinventars | `version-sources.yaml` | nur nach ausdrücklicher Bestätigung |
| Projekteigene Regel, Rezept, Check, Command, Skill | `rules/`, `reviews/`, `checks/`, `commands/`, `skills/` | von Hand |
| Projekteigene Vorgabe | `guidelines/<thema>.md` | von Hand |
| Projekteigene Instruktionsebene | `k-playbook.md` | von Hand |

**Todos gehen ausschließlich über `/k-todo`, nie von Hand.** Ihre Ablage
`data/todos.json` gehört dem Werkzeug: gelesen und geschrieben wird sie über den
Command, über das Subkommando `k-playbook todo` oder über die Oberfläche. Wer sie
selbst öffnet, führt eine zweite Auslegung des Formats ein, die in keine Prüfung
eingeht. Die einzige Ausnahme ist die Auflösung eines Merge-Konflikts — beide
Einträge behalten, `nextId` auf `max(id)+1` setzen —, und die macht ein Mensch.
Eine ältere `TODO.md` wird beim ersten Zugriff übersetzt und danach entfernt.

`docs/` ist nach Herkunft geteilt, nicht nach Thema. Das Unterverzeichnis sagt, wer
die Datei erzeugt hat und was sie veralten lässt: `code/` folgt dem Code, `libs/` den
Libraries, `versions/` den deklarierten Versionen, `extracted/` bleibt der Stand von
damals, `manual/` folgt niemandem außer der Hand, die sie schreibt. Wer eine Datei
in die falsche Herkunft legt, hängt sie an den falschen Nachzug.

## Namen

- **Tasks** tragen eine dreistellige Nummer, fortlaufend über `tasks/` **und**
  `tasks/done/` hinweg: `047-basis-tools-review-fixes.md`.
- **Doku aus Erzeugern** trägt eine zweistellige Nummer für die Lesereihenfolge:
  `00-overview.md`, `01-stack.md`.
- **Review-Rezepte** heißen `review-<name>.md`, sonst findet der Katalog sie nicht.
- **Läufe** tragen das Datum als `YYYY-MM-DD`, nie ein anderes Format: nur so sortiert
  das Verzeichnis chronologisch.
- **Datei- und Verzeichnisnamen bleiben ASCII**, auch wenn der Text darin Umlaute
  hat. Der Grund steht in `k-playbook/docs/writing-style.md`.

## Was nirgends hingehört

- **Ein Plan als Datei.** Eine Umsetzung, die über einen trivialen Einzelschritt
  hinausgeht, wird eine Task — nicht `plan.md`, nicht `konzept.md` neben dem Code.
- **Ergebnisse in `checks/`.** Dort liegen ausführbare Checks. Ein Ergebnis daneben
  sieht beim nächsten Lauf wie ein Skript aus.
- **Rohmaterial in `docs/`.** Was noch nicht ausgewertet ist, liegt in `material/`
  und wird nicht indiziert. Ausgewertet landet es in `docs/extracted/`, mit Quelle
  und Konfidenz.
- **Eine zweite Doku-Wurzel.** Projektwissen für AI-Sessions hat genau einen Einstieg:
  `docs/README.md`. Eine weitere Sammlung daneben wird nicht gefunden und veraltet
  unbemerkt.
- **Handkorrekturen an erzeugten Dateien.** `docs/README.md` und
  `docs/versions/inventory.md` werden von ihrem Erzeuger neu geschrieben; eine
  Korrektur von Hand ist beim nächsten Lauf weg.

## Was nicht ins Repository gehört

`results/` ist vollständig lokal: ein Review ist aus dem Code wiederholbar, sein
Ergebnis ist ein Stand von einer Maschine zu einem Zeitpunkt. Rohausgaben von
Secret-Scannern enthalten gefundene Geheimnisse im Klartext und dürfen nie eingecheckt
werden.

`cache/` ist ebenfalls lokal, aus einem anderen Grund: was dort liegt, ist aus dem
Projekt abgeleitet und jederzeit neu baubar. Ableitbares im Repository veraltet
unbemerkt. Wer etwas Unwiederbringliches ablegen will, braucht ein anderes Verzeichnis —
`cache/` darf ohne Rückfrage gelöscht werden.

`data/` gehört dagegen ins Repository: dort liegen Maschinendateien, die zum
Projektstand gehören, allen voran `todos.json`.

Bei `priv/` und `material/` entscheidet das Projekt. Was gerade gilt, zeigt die
Oberfläche im Block **Lokale Einstellungen** — gemessen mit `git check-ignore`, nicht
geraten.

## Einen Ablageort ergänzen

Ein Command, der einen neuen Ablageort einführt, trägt ihn **hier** ein, bevor er ihn
benutzt. Ein Ort, den nur ein Command-Text kennt, ist für jeden anderen unsichtbar;
der zweite Command legt dann etwas Ähnliches woanders ab, und ab da gibt es zwei
Wahrheiten.

Braucht ein Projekt einen anderen Ort, ersetzt es diese Datei durch eine gleichnamige
unter `k-playbook-local/guidelines/betrieb.md`. Einzelne Zeilen daraus zu übernehmen
ist nicht vorgesehen — die Datei gilt als Ganzes.
