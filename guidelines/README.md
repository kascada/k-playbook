# Guidelines

Verbindliche Vorgaben für den Betrieb eines k-playbook-Projekts: Konventionen, die
mehr als ein Command betreffen und deshalb nicht in einem einzelnen Command-Text
stehen dürfen.

Eine Guideline sagt, **wie** gearbeitet wird. Eine Regel unter `rules/` sagt, **was**
bei einer Änderung zusätzlich zu tun ist, und ein Review-Rezept unter `reviews/`
sagt, **wonach** gesucht wird. Wer eine Vorgabe sucht, die keinem dieser beiden
Zwecke dient, findet sie hier.

## Zusammenführung

Die Dateien hier werden ausgeliefert und gelten in jedem Projekt. Daneben steht
`k-playbook-local/guidelines/` für das, was nur ein Projekt betrifft.

Gelesen wird zuerst die mitgelieferte Datei, danach die projekteigene. Eine
gleichnamige projekteigene Datei **ersetzt** die mitgelieferte vollständig — dieselbe
Overlay-Regel wie bei `rules/`, `reviews/` und `checks/`. Eine leere projekteigene
Datei schaltet die mitgelieferte ab; ihr Inhalt darf dann den Grund nennen.

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
