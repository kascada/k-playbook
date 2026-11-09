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
abgeleitet — im Briefing dieselben Felder unter `context` —: `local.dir` für alles
Projekteigene, `playbook.dir` für die Installation. Jeder Ort unten leitet sich aus
`local.dir` ab, nie aus einer Konfiguration und nie durch Suchen. Ein fest verdrahteter
Pfad geht in dem Moment kaputt, in dem ein Projekt anders heißt oder tiefer liegt. Für
Text, den ein Mensch liest, bleibt die kurze Schreibweise `k-playbook-local/...` richtig;
zum Schreiben wird der aufgelöste Pfad benutzt.

**Ein fehlendes Verzeichnis wird nie still angelegt.** Fehlt ein Ort aus der Tabelle
unten, wird gefragt, ob genau dieses Verzeichnis angelegt oder `/k-gui` gestartet werden
soll, das die projektlokale Struktur wiederherstellt. Nicht hart abbrechen, nicht still
anlegen, nie einen anderen Pfad nehmen. Das gilt für die Verzeichnisse, die das
Einrichten anlegt. Ausgenommen sind die Zeilen mit „entsteht beim ersten Lauf von …“:
Dort ist ein fehlendes Verzeichnis der normale Zustand vor dem ersten Lauf, und der
erzeugende Command legt es ohne Rückfrage selbst an.

## Was wohin

Alle Pfade relativ zu `local.dir`, also `k-playbook-local/`.

| Was | Wohin | Wer legt es ab |
|---|---|---|
| Task, offen | `keph:/user/~/tasks/<NNN>-<slug>.md` — in der Projekt-Collection, ein Dokument je Task, nicht unter `local.dir` | `/k-task-create`, `/k-remediation` über `k_playbook_doc_create` und Kephalaion `write`; Form nach `k-playbook:/instructions/on-demand/tasks.md` |
| Task, erledigt | `keph:/user/~/tasks/done/<NNN>-<slug>.md` — in der Projekt-Collection, nicht unter `local.dir` | `/k-task-run` verschiebt sie dorthin mit `k_playbook_doc_done` |
| Kurznotiz, unsortiert | `todos/` in der Projekt-Collection, nicht unter `local.dir` | `/k-todo`, `k-playbook todo`, die Oberfläche |
| Review- und Audit-Lauf | `results/<YYYY-MM-DD>/` bzw. `results/<familie>/<YYYY-MM-DD>/` | `/k-review`, `/k-audit` |
| Lauf-Protokoll | `results/log.md` | `/k-review`, `/k-audit` |
| Bewusste Entscheidung zu einem Befund | `known-decisions.md` | von Hand |
| Befund aus Analyse oder Fehlersuche | `keph:/queue/findings/<NNN>-<slug>.md` — in der Projekt-Collection, ein Dokument je Befund, nicht unter `local.dir` | die Sitzung selbst über `k_playbook_doc_create` und Kephalaion `write`, Nachträge von `/k-danke`; Form nach `k-playbook:/instructions/on-demand/befunde.md` |
| Stand eines laufenden Vorhabens | `keph:/user/~/workbench/<NNN>-<slug>.md` — in der Projekt-Collection, nicht unter `local.dir` | die Sitzung selbst über `k_playbook_doc_create` und Kephalaion `write`; Form nach der Anweisung „Die Workbench“ |
| Doku aus dem Code | `keph:/docs/generated/code/<NN>-<slug>.md` — in der Projekt-Collection, nicht unter `local.dir`; dazwischen das rechnerlokale Arbeitsverzeichnis `.k-playbook/work/docs/code/`, ohne Kephalaion gibt es keine | `/k-docs-code` und der Skill `ks-overlay-repo-analyse`, nur über `k-playbook docs checkout\|publish code` |
| Steckbrief zu Library oder Tool | `keph:/docs/generated/libs/<name>.md` — in der Projekt-Collection, nicht unter `local.dir`; dazwischen das rechnerlokale Arbeitsverzeichnis `.k-playbook/work/docs/libs/`, ohne Kephalaion gibt es keine | `/k-docs-tools`, nur über `k-playbook docs checkout\|publish libs` |
| Versionsinventar | `keph:/docs/generated/versions/inventory.md` — in der Projekt-Collection, nicht unter `local.dir`; entsteht beim ersten Lauf von `/k-doc-inventory`, ohne Kephalaion gibt es kein Inventar | `/k-doc-inventory`, `k-playbook inventory` |
| Handgeschriebene Doku | `keph:/docs/manual/<slug>.md` — in der Projekt-Collection, nicht unter `local.dir` | von Hand, über Kephalaion `create` bzw. `write` |
| Doku-Index | `keph:/docs/README.md` und `index.md` je Verzeichnis — erzeugt Kephalaion | kein Command |
| Notiz, Zwischenstand, Privates | `priv/` | von Hand |
| Quellen des Versionsinventars | `version-sources.yaml` | nur nach ausdrücklicher Bestätigung |
| Projekteigenes Rezept | `reviews/` | von Hand |
| Projekteigener Command | `.k-playbook/commands/` unter `project.dir`, nicht unter `local.dir` — versioniert; das Einrichten legt es nicht an, es entsteht mit dem ersten eigenen Command | von Hand |
| Projekteigener Skill | `.k-playbook/skills/` unter `project.dir`, nicht unter `local.dir` — versioniert; das Einrichten legt es nicht an, es entsteht mit dem ersten eigenen Skill | von Hand |
| Projekteigene Vorgabe | `guidelines/<thema>.md` | von Hand |
| Projekteigene Instruktionsebene | `k-playbook.md` | von Hand |
| Check-Runner, Skripte, Werkzeugmatrizen | `<playbook.dir>/bin/k-check`, `~/.local/share/k-playbook/scripts/`, `~/.local/share/k-playbook/scripts/security-tools.tsv` — die eine Ausnahme von `local.dir`: nur lesen und aufrufen, nie schreiben | k-playbook |

**Todos gehen ausschließlich über `/k-todo`, nie von Hand.** Ihre Ablage, die
Dokumente unter `todos/` in der Projekt-Collection, gehört dem Werkzeug: gelesen und
geschrieben wird sie über den Command, über das Subkommando `k-playbook todo` oder über
die Oberfläche. Wer sie selbst öffnet, führt eine zweite Auslegung des Formats ein, die
in keine Prüfung eingeht. Ein `data/todos.json` von früher übernimmt
`k-playbook todo migrate`; eine ältere `TODO.md` wird nur noch gemeldet.

Die Projektdoku ist nach Herkunft geteilt, nicht nach Thema — in `keph:/docs/` wie in
dem, was unter `docs/` noch auf der Platte liegt. Das Unterverzeichnis sagt, wer die
Datei erzeugt hat und was sie veralten lässt: `code/` folgt dem Code, `libs/` den
Libraries (beide noch auf der Platte, bis ihre Erzeuger umgestellt sind),
`generated/versions/` den deklarierten Versionen, `manual/` folgt niemandem außer der
Hand, die sie schreibt (beide in der Collection). Ein `versions/`, `manual/` oder
`extracted/` auf der Platte ist ein Stand von damals; `/k-upgrade` (3.2) bringt ihn nach
Kephalaion oder räumt ihn ab. Wer eine Datei in die falsche Herkunft legt, hängt sie an
den falschen Nachzug.

## Namen

- **Tasks** tragen eine dreistellige Nummer, die k-playbook beim Anlegen vergibt; die
  Nummernregel steht in `k-playbook:/instructions/on-demand/tasks.md`, nur dort.
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
- **Ein projekteigener Check in `checks/`.** Das Verzeichnis gibt es nicht mehr (Task 134):
  `k-check` führt nur die mitgelieferten Checks aus, ein Skript dort läuft nie.
  `/k-upgrade` räumt es ab.
- **Eine zweite Doku-Wurzel.** Projektwissen für AI-Sessions hat genau einen Einstieg:
  `keph:/docs/`, das Briefing nennt ihn unter `Projektdoku`. Eine weitere Sammlung
  daneben — auch ein Index auf der Platte — wird nicht gefunden und veraltet unbemerkt.
- **Handkorrekturen an erzeugten Dateien.** `keph:/docs/README.md` und
  `keph:/docs/generated/versions/inventory.md` werden von ihrem Erzeuger neu geschrieben;
  eine Korrektur von Hand ist beim nächsten Lauf weg.

## Was nicht ins Repository gehört

`results/` ist vollständig lokal: ein Review ist aus dem Code wiederholbar, sein
Ergebnis ist ein Stand von einer Maschine zu einem Zeitpunkt. Rohausgaben von
Secret-Scannern enthalten gefundene Geheimnisse im Klartext und dürfen nie eingecheckt
werden.

`cache/` ist ebenfalls lokal, aus einem anderen Grund: was dort liegt, ist aus dem
Projekt abgeleitet und jederzeit neu baubar. Ableitbares im Repository veraltet
unbemerkt. Wer etwas Unwiederbringliches ablegen will, braucht ein anderes Verzeichnis —
`cache/` darf ohne Rückfrage gelöscht werden.

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
