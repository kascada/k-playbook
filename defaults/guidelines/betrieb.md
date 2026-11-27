# Betrieb: Wo abgelegte Dateien landen

Diese Guideline legt fest, wohin eine Datei gehört, die im Lauf der Arbeit entsteht —
eine Task, ein Befund, eine Doku-Seite, eine Notiz. Sie ist die
eine Auskunft dazu. Ein Command, ein Skill oder eine Regel, die eine Datei ablegt,
verweist hierher, statt den Ort ein zweites Mal zu begründen.

## Zwei Grundsätze

**Mitgeliefertes wird nie geändert.** Was k-playbook liefert, liegt nicht im Projekt: Die
Texte kommen über `k-playbook:`-Verweise, Commands, Skills und Skripte liegen unter
`~/.local/share/k-playbook/` und gehören dem Programm. Was dort abgelegt oder geändert
wird, ist nach der nächsten Aktualisierung weg — ohne Fehlermeldung, ohne Spur. Alles, was
das Projekt hervorbringt, liegt im Projekt selbst oder in seiner Collection.

**Pfade werden nicht geraten.** Jeder Ort, an dem etwas abgelegt wird, ist ein
`keph:`-Verweis oder liegt unter der Projektwurzel, dort unter `.k-playbook/`. Die
Projektwurzel ist `project.dir` aus dem Briefing, nie ein Wert aus einer Konfiguration und nie
das Ergebnis einer Suche. Ein fest verdrahteter Pfad geht in dem Moment kaputt, in dem ein Projekt
anders heißt oder tiefer liegt. Für Text, den ein Mensch liest, bleibt die kurze
Schreibweise `.k-playbook/...` richtig; zum Schreiben wird der aufgelöste Pfad benutzt.

## Was wohin

| Was | Wohin | Wer legt es ab |
|---|---|---|
| Task, offen | `keph:/user/~/tasks/<NNN>-<slug>.md` — in der Projekt-Collection, ein Dokument je Task | `/k-task-create` über `k_playbook_task_create`, `/k-remediation` (abgeschaltet) bis zum Umbau über `k_playbook_doc_create` und Kephalaion `write`; Form nach `k-playbook:/instructions/on-demand/tasks.md` |
| Task, erledigt | `keph:/user/~/tasks/done/<NNN>-<slug>.md` — in der Projekt-Collection | `/k-task-run` verschiebt sie dorthin mit `k_playbook_doc_done` |
| Kurznotiz, unsortiert | `todos/` in der Projekt-Collection | `/k-todo`, `k-playbook todo`, die Oberfläche |
| Meldung: Fehler oder Wunsch, den jemand anderes erledigt (an k-playbook, Kephalaion oder das Projekt) | `keph:/feedback/<NNN>-<slug>.md` — in der Projekt-Collection, projektweit, ein Dokument je Meldung; erledigt und abgelehnt unter `keph:/feedback/done/` | die Sitzung oder der Mensch über `k_playbook_feedback_create` bzw. `k-playbook feedback create` und `/k-feedback`; Status über `k_playbook_feedback_update` bzw. `k-playbook feedback update` |
| Befund aus Analyse oder Fehlersuche | `keph:/queue/findings/<NNN>-<slug>.md` — in der Projekt-Collection, ein Dokument je Befund | die Sitzung selbst über `k_playbook_doc_create` und Kephalaion `write`, Nachträge von `/k-danke`; Form nach `k-playbook:/instructions/on-demand/befunde.md` |
| Doku-Auftrag (Nachzug der erzeugten Doku) | `keph:/queue/docs/<NNN>-<slug>.md` — in der Projekt-Collection, ein Dokument je Auftrag; erledigt unter `keph:/queue/docs/done/` | jede Sitzung nach einer Codeänderung über `k_playbook_docs_order_add`, `/k-docs`; die zerlegten Aufträge `/k-docs-code` über `k_playbook_docs_orders_split`, das sie auch abschließt (`k_playbook_doc_done`); Form nach `k-playbook:/instructions/on-demand/doku-auftraege.md` |
| Protokoll eines Laufs von `/k-befunde-einordnen` | `keph:/queue/einordnung/<NNN>-<datum>-<sitzung>.md` — in der Projekt-Collection, ein Dokument je Lauf | `/k-befunde-einordnen` |
| Befund, den die Einordnung nicht zuordnen kann | `keph:/queue/unclear/<dateiname>` — in der Projekt-Collection | `/k-befunde-einordnen` |
| Stand eines laufenden Vorhabens | `keph:/user/~/workbench/<NNN>-<slug>.md` — in der Projekt-Collection | die Sitzung selbst über `k_playbook_doc_create` und Kephalaion `write`; Form nach der Anweisung „Die Workbench“ |
| Ergebnis, das der Nutzer mittelfristig sichern und zeigen will | `keph:/results/<YYYY-MM-DD>-<kurzname>.md` — in der Projekt-Collection, projektweit, nur `.md` | nur auf Wunsch des Nutzers oder wenn ein Command es ausdrücklich vorsieht, nie als Pflicht; über Kephalaion `create`, aufgeräumt vom Nutzer; Form nach `k-playbook:/instructions/on-demand/ergebnisse.md` |
| Ergebnis eines Bericht-Laufs von `/k-review` | `keph:/results/review/<YYYY-MM-DD>-<rezept>.md` — in der Projekt-Collection, projektweit, nur die Ergebnisdatei `review-triage.md`, nie SARIF oder Beleg | `/k-review` bei jedem Bericht-Lauf über Kephalaion `create`, nie überschreibend; abgehakt vom Nutzer oder auf seinen Wunsch von einer Sitzung mit Kephalaion `write`; Form nach `k-playbook:/instructions/on-demand/ergebnisse.md` |
| Doku aus dem Code | `keph:/docs/generated/code/<NN>-<slug>.md` — in der Projekt-Collection; dazwischen das rechnerlokale Arbeitsverzeichnis `.k-playbook/work/docs/code/`, ohne Kephalaion gibt es keine | `/k-docs-code` und der Skill `ks-overlay-repo-analyse`, nur über `k-playbook docs checkout\|publish code` |
| Steckbrief zu Library oder Tool | `keph:/docs/generated/libs/<name>.md` — in der Projekt-Collection; dazwischen das rechnerlokale Arbeitsverzeichnis `.k-playbook/work/docs/libs/`, ohne Kephalaion gibt es keine | `/k-docs-tools`, nur über `k-playbook docs checkout\|publish libs` |
| Versionsinventar | `keph:/docs/generated/versions/inventory.md` — in der Projekt-Collection; entsteht beim ersten Lauf von `/k-doc-inventory`, ohne Kephalaion gibt es kein Inventar | `/k-doc-inventory`, `k-playbook inventory` |
| Handgeschriebene Doku | `keph:/docs/manual/<slug>.md` — in der Projekt-Collection | von Hand, über Kephalaion `create` bzw. `write` |
| Doku-Index | `keph:/docs/README.md` und `index.md` je Verzeichnis — erzeugt Kephalaion | kein Command |
| Quellen des Versionsinventars | `keph:/k-playbook/inventory.md` — in der Projekt-Collection; Einstellungen im Frontmatter, nur Pfade unterhalb der Projektwurzel | `/k-doc-inventory` nur nach ausdrücklicher Bestätigung und nur ergänzend, sonst von Hand |
| Anbindung an die Kephalaion-Collection | Block `kephalaion:` mit `hub` und `collection` in `.k-playbook/K-PLAYBOOK.yaml` unter `project.dir` — versioniert und für alle Nutzer des Projekts gleich; den Account wählt Kephalaion je Rechner (`kephalaion node client`) | der Einrichtungsschritt der Oberfläche nach bestandenem Test, sonst von Hand; danach committen |
| Projekteigenes Rezept | `keph:/k-playbook/reviews/review-<name>.md` — in der Projekt-Collection | von Hand, über Kephalaion `create` bzw. `write` |
| Projekteigener Command | `.k-playbook/commands/` unter `project.dir` — versioniert; das Einrichten legt es nicht an, es entsteht mit dem ersten eigenen Command | von Hand |
| Projekteigener Skill | `.k-playbook/skills/` unter `project.dir` — versioniert; das Einrichten legt es nicht an, es entsteht mit dem ersten eigenen Skill | von Hand |
| Projekteigene Vorgabe | `keph:/docs/guidelines/<thema>.md` — in der Projekt-Collection | von Hand, über Kephalaion `create` bzw. `write` |
| Projekteigene Anweisung | `keph:/k-playbook/instructions/always/<nummer>-<thema>.md` (gilt in jeder Sitzung) oder `keph:/k-playbook/instructions/on-demand/<thema>.md` (auf Abruf) — in der Projekt-Collection | von Hand, über Kephalaion `create` bzw. `write` |
| Skripte, Werkzeugmatrizen | `~/.local/share/k-playbook/scripts/`, `~/.local/share/k-playbook/scripts/security-tools.tsv` — kein Ablageort, sie gehören dem Programm: nur lesen und aufrufen, nie schreiben | k-playbook |

**Todos gehen ausschließlich über `/k-todo`, nie von Hand.** Ihre Ablage, die
Dokumente unter `todos/` in der Projekt-Collection, gehört dem Werkzeug: gelesen und
geschrieben wird sie über den Command, über das Subkommando `k-playbook todo` oder über
die Oberfläche. Wer sie selbst öffnet, führt eine zweite Auslegung des Formats ein, die
in keine Prüfung eingeht.

**Meldungen gehen ausschließlich über die Feedback-Werkzeuge, nie von Hand.** Ihre Ablage,
die Dokumente unter `feedback/` in der Projekt-Collection, gehört dem Programm wie die der
Todos: gelesen und geschrieben wird sie über `/k-feedback`, die MCP-Werkzeuge
`k_playbook_feedback_*` oder das Subkommando `k-playbook feedback`. Den Kopf einer Meldung
setzt das Werkzeug; eine Meldung ist kein Befund und wartet nicht in der Queue, sondern
bleibt bis zur Erledigung liegen.

Die Ergebnisse von `/k-review` legt der Command selbst unter `keph:/results/review/` ab; Fundliste
und Beleg bleiben im Arbeitsverzeichnis des Laufs. Der Ort für die Ergebnisse von `/k-audit` ist
noch offen; bis dahin pausiert der Audit.

Die Projektdoku ist nach Herkunft geteilt, nicht nach Thema. Das Unterverzeichnis von
`keph:/docs/` sagt, wer die Datei erzeugt hat und was sie veralten lässt:
`generated/code/` folgt dem Code, `generated/libs/` den Libraries, `generated/versions/`
den deklarierten Versionen, `manual/` folgt niemandem außer der Hand, die sie schreibt —
auch `manual/handbook/`, das Handbuch des Projekts, das die Oberfläche unter einem eigenen
Menüpunkt zeigt. Alle liegen in der Collection. Wer eine Datei in die falsche Herkunft
legt, hängt sie an den falschen Nachzug.

## Namen

- **Tasks** tragen eine dreistellige Nummer, die k-playbook beim Anlegen vergibt; die
  Nummernregel steht in `k-playbook:/instructions/on-demand/tasks.md`, nur dort.
- **Doku aus Erzeugern** trägt eine zweistellige Nummer für die Lesereihenfolge:
  `00-overview.md`, `01-stack.md`.
- **Review-Rezepte** heißen `review-<name>.md`, sonst findet der Katalog sie nicht.
- **Läufe** tragen das Datum als `YYYY-MM-DD`, nie ein anderes Format: nur so sortiert
  das Verzeichnis chronologisch.
- **Datei- und Verzeichnisnamen bleiben ASCII**, auch wenn der Text darin Umlaute
  hat. macOS speichert Umlaute zerlegt, Linux zusammengesetzt; derselbe Name wäre auf zwei
  Rechnern nicht dieselbe Zeichenkette.

## Was nirgends hingehört

- **Ein Plan als Datei.** Eine Umsetzung, die über einen trivialen Einzelschritt
  hinausgeht, wird eine Task — nicht `plan.md`, nicht `konzept.md` neben dem Code.
- **Ein projekteigener Check.** k-playbook führt keine Checks aus (Task 155): Ein Skript,
  das als Check abgelegt wird, läuft nie.
- **Eine zweite Doku-Wurzel.** Projektwissen für AI-Sessions hat genau einen Einstieg:
  `keph:/docs/`, das Briefing nennt ihn unter `Projektdoku`. Eine weitere Sammlung
  daneben — auch ein Index auf der Platte — wird nicht gefunden und veraltet unbemerkt.
- **Handkorrekturen an erzeugten Dateien.** `keph:/docs/README.md` und
  `keph:/docs/generated/versions/inventory.md` werden von ihrem Erzeuger neu geschrieben;
  eine Korrektur von Hand ist beim nächsten Lauf weg.

Rohausgaben von Secret-Scannern enthalten gefundene Geheimnisse im Klartext und werden
nie eingecheckt.

## Einen Ablageort ergänzen

Ein Command, der einen neuen Ablageort einführt, trägt ihn **hier** ein, bevor er ihn
benutzt. Ein Ort, den nur ein Command-Text kennt, ist für jeden anderen unsichtbar;
der zweite Command legt dann etwas Ähnliches woanders ab, und ab da gibt es zwei
Wahrheiten.

Diese Guideline gibt es nur mitgeliefert. Projekteigene Vorgaben liegen unter
`keph:/docs/guidelines/`.
