# instructions/always

Die Teile hier gelten in jeder Sitzung. k-playbook baut sie zu einem Text zusammen und
liefert ihn unter `k-playbook:/instructions/always/` aus; das Briefing trägt denselben Text
vorweg (`documents.always.content`), eine Sitzung muss ihn also nicht eigens lesen.

## Zusammenbauen

- Aufgenommen werden nur Dateien dieses Verzeichnisses, deren Name mit einer Ziffer beginnt,
  nicht aber Unterverzeichnisse und nicht diese `README.md`. Eine Datei ohne Ziffer vorn wird
  nicht aufgenommen und als Problem gemeldet.
- Mitgelieferte und lokale Teile (`k-playbook-local/instructions/always/`) werden gemeinsam
  nach der Ziffer am Anfang des Namens sortiert, und zwar numerisch: `20-` kommt vor `100-`.
  Bei gleicher Ziffer entscheidet der Name.
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
  Felder, nichts selbst lesen, Ablegen nur aus `local.dir`. Steht gleich nach den Verweisen,
  weil das Briefing das Erste ist, was eine Sitzung liest; Nachschlagbares dazu liegt in
  `on-demand/briefing-felder.md`.
- `100-projektwissen-zuerst.md`: Zuerst die Projektdoku lesen, dann den Code.
- `200-befunde-festhalten.md`: Erkenntnisse aus Analyse und Fehlersuche festhalten.
- `210-workbench.md`: Den Stand eines laufenden Vorhabens in der Workbench führen. Steht
  hinter den Befunden, weil beide sagen, was eine Sitzung während der Arbeit festhält.
