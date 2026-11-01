---
title: Verweise nachziehen
description: Was in einem Task, Command oder einer Anweisung als Verweis gilt, welches Präfix ein nackter Pfad oder relativer Link bekommt, was unberührt bleibt, wie nachgezogen wird und was ein Check daran findet — die eine Stelle für /k-task-refine, /k-upgrade, check_verweise und jede Sitzung.
---

## Verweise nachziehen

Die Form eines Verweises steht in `k-playbook:/instructions/always/010-verweise.md`: ein
einziger Pfad mit Präfix (`project:/`, `k-playbook:/`, `keph:/`). Diese Anweisung sagt, wie
eine Stelle, die diese Form noch nicht hat, erkannt und nachgezogen wird. Sie ist die eine
Stelle dafür: `/k-task-refine`, `/k-upgrade` (Umstellung 3.3), der Check `check_verweise.sh`,
spätere Checks und jede Sitzung richten sich nach ihr und wiederholen sie nicht.

### Was ein Verweis ist

Ein Verweis ist eine Stelle, die der Leser öffnen und lesen soll:

- jeder Eintrag unter `## Referenzen`,
- jeder Markdown-Link (`[text](ziel)`),
- jede Lese-Anweisung: „lies …“, „siehe …“, ein Ziel, das ein Schritt lesen oder befolgen
  lässt, und die Quellen, auf die sich der Intent eines Tasks stützt.

Kein Verweis ist

- die Ortsangabe einer Datei, die erzeugt oder geändert wird („schreibt nach
  `k-playbook-local/…`“, „in `defaults/commands/x.md` ändern“),
- ein Dateiname im Fließtext, der nur etwas benennt („die Datei `AGENTS.md`“),
- ein Pfad in einem Code-Block, den ein Befehl als Argument bekommt.

Wo eine Ortsangabe zugleich gelesen werden soll — etwa unter `## Referenzen` eines Tasks, der
die Datei ändert —, ist sie ein Verweis.

### Welches Präfix ein nackter Pfad bekommt

| Der Pfad meint | Präfix |
|---|---|
| eine Datei im Projekt, ab der Projektwurzel | `project:/<pfad>` |
| einen Text, den k-playbook liefert (Instruction, Rule, Review, Guideline, Command) | `k-playbook:/<sorte>/<pfad>` |
| ein Dokument der Projekt-Collection | `keph:/<pfad>` |

Ein relativer Link (`../x.md`, `./x.md`) wird erst gegen den Ort des Dokuments aufgelöst und
dann wie ein nackter Pfad behandelt. Ein Pfad mit `k-playbook-local/<sorte>/…` meint die
lokale Fassung; gelesen wird sie trotzdem als `k-playbook:/<sorte>/…`, weil k-playbook die
Überlagerung rechnet.

**Mehrdeutig** ist ein Pfad, der zu zwei Präfixen passt. Der häufigste Fall steht im
k-playbook-Repo selbst: `defaults/commands/x.md` ist

- `k-playbook:/commands/x.md`, wenn der Text gelesen werden soll, wie er ausgeliefert wird,
- `project:/defaults/commands/x.md`, wenn die Quelle gemeint ist, die geändert wird.

Ebenso `rules/x.md` (`k-playbook:/rules/x.md` oder die Repo-Kopie `project:/rules/x.md`) und
`tasks/…` (ein Task in `keph:/user/<user>/tasks/…` oder eine alte Datei auf der Platte). Was
sich aus dem Satz um den Pfad nicht eindeutig zuordnen lässt, wird erfragt, nicht geraten.

### Was unberührt bleibt

Belege werden nicht umgeschrieben, auch wenn dort nackte Pfade oder alte Plattenpfade stehen:

- ein `## Review-Log`, `## Ausführung` und alles, was ein Lauf danach angehängt hat (Diff,
  Review, Statuszeilen),
- Diffs und zitierte Ausgaben,
- erledigte Tasks unter `done/`.

### Wie nachgezogen wird

- **In einem Task** mit `k_playbook_doc_replace` (`old` genau ein Vorkommen, `new`) und
  `since_revision`, nach dem Schreibweg in `k-playbook:/instructions/on-demand/tasks.md`
  („Lesen und schreiben“).
- **In einer Datei auf der Platte** mit dem Edit-Werkzeug des Assistenten.

Jede Ersetzung ändert nur den Verweis, nicht den Satz um ihn. Bei einem Markdown-Link wird
das Ziel ersetzt; der Linktext bleibt.

### Check und KI

Ein Check sieht nur den Text. Er erkennt

- relative Markdown-Links, deren Ziel mit `../` oder `./` beginnt, überall,
- Pfade ohne Präfix in Backticks, die mit einer bekannten Wurzel des Projekts beginnen, wenn
  sie unter `## Referenzen` stehen oder in einer Zeile mit Lese-Anweisung (`lies`, `liest`,
  `lesen`, `siehe`).

Er erkennt nicht, ob ein Pfad in anderem Fließtext gelesen werden soll, und nicht, welches von
zwei Präfixen gemeint ist. Der Check findet und meldet; er korrigiert nicht. Korrigiert wird
von einer KI nach dieser Anweisung. Eine gemeldete Stelle, die doch eine Ortsangabe ist, kommt
mit Grund in die Ausnahmeliste des Checks.
