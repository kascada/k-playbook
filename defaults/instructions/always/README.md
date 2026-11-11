# instructions/always

Die Teile hier gelten in jeder Sitzung. k-playbook baut sie zu einem Text zusammen und
liefert ihn unter `k-playbook:/instructions/always/` aus; das Briefing trägt denselben Text
vorweg (`documents.always.content`), eine Sitzung muss ihn also nicht eigens lesen.

## Zusammenbauen

- Aufgenommen werden nur Dateien dieses Verzeichnisses, deren Name mit einer Ziffer beginnt,
  nicht aber Unterverzeichnisse und nicht diese `README.md`. Eine Datei ohne Ziffer vorn wird
  nicht aufgenommen und als Problem gemeldet.
- Mitgelieferte und lokale Teile werden gemeinsam
  nach der Ziffer am Anfang des Namens sortiert, und zwar numerisch: `20-` kommt vor `100-`.
  Bei gleicher Ziffer entscheidet der Name. Die lokalen Teile liegen in der Collection des
  Projekts unter `keph:/k-playbook/instructions/always/`; `k-playbook-local/instructions/always/`
  wird nicht mehr gelesen.
- Ein gleichnamiger lokaler Teil ersetzt den mitgelieferten vollständig. Ein leerer lokaler
  Teil schaltet ihn ab. Leer heißt: nur Leerzeilen und Zeilen, die mit `#` beginnen — auch
  eine Datei nur aus Überschriften schaltet also ab.
- Das Frontmatter eines Teils wird vor dem Zusammenfügen entfernt.

## Die Ziffer gehört zum Namen

Überlagert wird am vollständigen Dateinamen, Ziffer eingeschlossen. Ein lokaler
`100-projektwissen-zuerst.md` ersetzt genau diesen Teil, ein lokaler
`150-projektwissen-zuerst.md` wäre ein neuer daneben. Deshalb bleiben die Nummern der
mitgelieferten Teile stabil; einen mitgelieferten Teil umzubenennen ist eine
Vertragsänderung. Verwaiste lokale Teile werden nicht erkannt.

## Nummernbereiche

- Mitgelieferte Teile tragen Nummern unter 1000.
- Lokale Teile tragen Nummern ab 1000 und stehen damit hinter allem Mitgelieferten. Wer einen
  lokalen Teil gezielt davor setzen will, wählt eine kleinere Nummer.

## Offene Stellen markieren

Eine Stelle, die später angepasst werden muss, bekommt einen HTML-Kommentar mit dem Marker
`NACHZIEHEN:`:

```markdown
<!-- NACHZIEHEN: was später anzupassen ist und warum -->
```

Der Kommentar steht direkt unter dem betroffenen Absatz und nie in einer Tabelle, weil er sie
sonst zerteilt. Alle offenen Stellen findet `grep -rn "NACHZIEHEN:"`.

## Teile

- `010-verweise.md`: Wie Verweise geschrieben werden und wie man sie erreicht. Steht als
  Erstes, weil alle weiteren Teile Verweise benutzen.
- `030-briefing.md`: Die Regeln jeder Sitzung zum Briefing — einmal laden, die Handvoll
  Felder, die Umgebungsprüfung `health` in der ersten Antwort nennen und nichts still beheben,
  nichts selbst lesen, Ablegen nur aus `local.dir`. Steht gleich nach den Verweisen,
  weil das Briefing das Erste ist, was eine Sitzung liest; Nachschlagbares dazu liegt in
  `on-demand/briefing-felder.md`.
- `100-projektwissen-zuerst.md`: Zuerst die Projektdoku lesen, dann den Code.
- `200-befunde-festhalten.md`: Erkenntnisse aus Analyse und Fehlersuche festhalten.
- `210-workbench.md`: Den Stand eines laufenden Vorhabens in der Workbench führen. Steht
  hinter den Befunden, weil beide sagen, was eine Sitzung während der Arbeit festhält.
- `220-regeln-anwenden.md`: Doku nachziehen — nach einer Änderung am Code Handdoku,
  `README.md` und `AGENTS.md` im selben Arbeitsgang nachziehen; für die erzeugte Doku einen
  Auftrag nach `on-demand/doku-auftraege.md` einreihen; den ausdrücklichen Bericht liefert
  `/k-enforcement`. Steht hinter Befunden und Workbench, weil auch er sagt, was eine Sitzung
  während der Arbeit festhält, und vor den Tasks, weil er für jede Änderung gilt, ob mit Task
  oder ohne.
- `300-arbeiten-in-tasks.md`: Was über einen trivialen Einzelschritt hinausgeht, wird ein
  Task in `keph:/user/~/tasks/` — `/k-task-create`, `/k-task-refine`, `/k-task-run`. Steht
  hinter Befunden und Workbench, weil es sagt, wie Arbeit ihren Weg nimmt, nachdem klar ist,
  was festgehalten wird; die Form eines Tasks liegt in `on-demand/tasks.md`.
- `400-github-ueber-run.md`: Netzbefehle von git und `gh` gegen das Repo eines Dienstes
  laufen über `k-playbook github run <dienst>`; ohne Dienst gilt das Bisherige. Steht als
  Letztes, weil es nicht sagt, wie Arbeit festgehalten oder geführt wird, sondern einen
  einzelnen Zugriffsweg regelt, den die Sitzung erst beim Ausführen braucht; Ausnahmen,
  Fehlerbild und Verbote liegen in `on-demand/github.md`.
