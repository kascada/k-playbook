# Guidelines

Verbindliche Vorgaben für den Betrieb eines k-playbook-Projekts: Konventionen, die
mehr als ein Command betreffen und deshalb nicht in einem einzelnen Command-Text
stehen dürfen.

Eine Guideline sagt, **wie** gearbeitet wird. Ein Review-Rezept unter `reviews/` sagt,
**wonach** gesucht wird. Wer eine Vorgabe sucht, die nicht der Suche eines Reviews dient,
findet sie hier.

## Zusammenführung

Die Dateien hier werden ausgeliefert und gelten in jedem Projekt. Was nur ein Projekt
betrifft, steht als Guideline in der Projekt-Collection unter `keph:/docs/guidelines/`.
Das frühere `k-playbook-local/guidelines/` entfällt; `/k-upgrade` löscht es. Wie
mitgelieferte und projekteigene Guidelines zusammenspielen, ist noch offen (Todo #40).

## Dateien

- `betrieb.md` — wo abgelegte Dateien landen: Ablageorte, Namensregeln und was
  nirgends hingehört.

## Eine Guideline ergänzen

Eine neue Guideline bekommt eine eigene Datei `guidelines/<thema>.md` und eine Zeile
in der Liste oben. Was in eine bestehende Guideline gehört, wird dort ergänzt statt
als zweite Datei danebengelegt — zwei Dateien zum selben Thema widersprechen sich
früher oder später.

Commands und Skills verweisen auf die Guideline, statt ihren Inhalt zu wiederholen.
Eine Vorgabe, die an zwei Stellen ausformuliert ist, veraltet an einer davon.
