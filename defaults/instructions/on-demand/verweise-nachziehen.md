---
title: Verweise nachziehen
description: Was in einem Task, Command oder einer Anweisung als Verweis gilt, welches Präfix ein nackter Pfad oder relativer Link bekommt, was unberührt bleibt, wie nachgezogen wird und was am Text allein erkennbar ist und was ein Urteil braucht — die eine Stelle für /k-task-refine und jede Sitzung.
---

## Verweise nachziehen

Die Form eines Verweises steht in `k-playbook:/instructions/always/010-verweise.md`: ein
einziger Pfad mit Präfix (`project:/`, `k-playbook:/`, `keph:/`). Diese Anweisung sagt, wie
eine Stelle, die diese Form noch nicht hat, erkannt und nachgezogen wird. Sie ist die eine
Stelle dafür: `/k-task-refine` und jede Sitzung richten sich nach ihr und wiederholen sie
nicht.

### Was ein Verweis ist

Ein Verweis ist eine Stelle, die der Leser öffnen und lesen soll:

- jeder Eintrag unter `## Referenzen`,
- jeder Markdown-Link (`[text](ziel)`),
- jede Lese-Anweisung: „lies …“, „siehe …“, ein Ziel, das ein Schritt lesen oder befolgen
  lässt, und die Quellen, auf die sich der Intent eines Tasks stützt.

Kein Verweis ist

- die Ortsangabe einer Datei, die erzeugt oder geändert wird („schreibt nach
  `.k-playbook/work/…`“, „in `defaults/commands/x.md` ändern“),
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
dann wie ein nackter Pfad behandelt.

Projekteigene Commands und Skills liegen in `.k-playbook/` (Task 129):

- `.k-playbook/commands/…` ist die lokale Fassung von `k-playbook:/commands/…` und wird so
  gelesen.
- Skills sind keine Sorte von `k-playbook:`. `.k-playbook/skills/…` wird
  `project:/.k-playbook/skills/…`.

**Mehrdeutig** ist ein Pfad, der zu zwei Präfixen passt. Der häufigste Fall steht im
k-playbook-Repo selbst: `defaults/commands/x.md` ist

- `k-playbook:/commands/x.md`, wenn der Text gelesen werden soll, wie er ausgeliefert wird,
- `project:/defaults/commands/x.md`, wenn die Quelle gemeint ist, die geändert wird.

Ebenso `.k-playbook/commands/x.md`: `project:/.k-playbook/commands/x.md`, wenn die Datei
gemeint ist, die geändert wird, und `k-playbook:/commands/x.md`, wenn der wirksame Text gelesen
werden soll. Ebenso `tasks/…` (ein Task in `keph:/user/<user>/tasks/…` oder eine Datei im
Projekt). Was sich aus dem Satz um den Pfad nicht eindeutig zuordnen lässt, wird erfragt, nicht
geraten.

### Was unberührt bleibt

Belege werden nicht umgeschrieben, auch wenn dort nackte Pfade oder alte Plattenpfade stehen:

- ein `## Review-Log`, `## Ausführung` und alles, was ein Lauf danach angehängt hat (Diff,
  Review, Statuszeilen),
- Diffs und zitierte Ausgaben,
- erledigte Tasks unter `done/`.

Ausgenommen sind Verweise auf Einträge der Queue (nächster Abschnitt): Sie werden auch in
Belegen eines offenen Tasks entfernt, nur unter `done/` nicht.

### Verweise auf die Queue

Ein Verweis auf einen Eintrag unter `keph:/queue/` (Befund oder Doku-Auftrag) bekommt kein
Präfix, er wird entfernt; die Regel steht in `k-playbook:/instructions/always/010-verweise.md`.
Dazu zählen auch Kurzformen wie `queue/findings/019`, „Befund 205 in `keph:/queue/findings/`“
oder ein bloßer Dateiname eines Befunds.

- Sagt der Satz nur, dass ein Befund geschrieben oder ein Auftrag eingereiht wurde, bleibt
  diese Aussage ohne den Verweis stehen.
- Braucht der Satz, was der Befund sagt, kommt dessen Kernaussage in eigenen Worten an die
  Stelle des Verweises. Ist der Befund noch lesbar, liest du ihn dafür; sonst bleibt nur die
  Aussage des Satzes.
- Die Ortsangabe eines Verzeichnisses (`keph:/queue/findings/` als Ablage für neue Befunde)
  bleibt.

### Wie nachgezogen wird

- **In einem Task** mit `k_playbook_doc_replace` (`old` genau ein Vorkommen, `new`) und
  `since_revision`, nach dem Schreibweg in `k-playbook:/instructions/on-demand/tasks.md`
  („Lesen und schreiben“).
- **In einer Datei auf der Platte** mit dem Edit-Werkzeug des Assistenten.

Jede Ersetzung ändert nur den Verweis, nicht den Satz um ihn. Bei einem Markdown-Link wird
das Ziel ersetzt; der Linktext bleibt.

### Am Text erkennbar und was ein Urteil braucht

Am Text allein erkennbar sind

- relative Markdown-Links, deren Ziel mit `../` oder `./` beginnt, überall,
- Pfade ohne Präfix in Backticks, die mit einer bekannten Wurzel des Projekts beginnen, wenn
  sie unter `## Referenzen` stehen oder in einer Zeile mit Lese-Anweisung (`lies`, `liest`,
  `lesen`, `siehe`).

In umzäunten Codeblöcken zählen Überschriften und Pfade ohne Präfix nicht: Ein `## Referenzen`
in einer Vorlage öffnet keinen Bereich, ein `# Kommentar` in einem Shell-Block beendet keinen.
Relative Links zählen weiter überall, auch in Codeblöcken.

Ein Urteil braucht, ob ein Pfad in anderem Fließtext gelesen werden soll und welches von zwei
Präfixen gemeint ist. Das entscheidet, wer nachzieht, nach dieser Anweisung; was sich nicht
eindeutig zuordnen lässt, wird erfragt. Eine Stelle, die nach dem Muster oben wie ein Verweis
aussieht, aber eine Ortsangabe ist, bleibt, wie sie ist.
