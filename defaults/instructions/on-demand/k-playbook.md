---
title: "Wie k-playbook aufgebaut ist"
description: "Was wo liegt (Programm, Hub, Projekt), wie Anweisungen zusammengebaut, überlagert und abgeschaltet werden, wohin die Teile einer AGENTS.md gehören und was in die Queue kommt — der Einstieg, bevor du über k-playbook Auskunft gibst oder eine Projektregel anlegst, änderst, verschiebst oder abschaltest; für Einzelheiten verweist er ins Handbuch."
---

## Wie k-playbook aufgebaut ist

Lies diese Anweisung, bevor du über k-playbook Auskunft gibst oder eine Projektregel anlegst,
änderst, verschiebst oder abschaltest. Sie sagt, was wo liegt, wie Anweisungen funktionieren,
wohin die Teile einer `AGENTS.md` gehören und was in die Queue kommt. Die ausführliche Referenz
ist das Handbuch (englisch); der letzte Abschnitt nennt die Seiten.

### Aufbau

k-playbook hat drei Orte. Was es mitliefert und was ein Projekt selbst besitzt, überschneidet
sich nie; nur das Zweite liegt im Projekt.

- **Das Programm.** `~/.local/bin/k-playbook`, eines je Rechner, für alle Projekte. Es entpackt
  Commands, Skills und Skripte nach `~/.local/share/k-playbook/`. Beides gehört dem Programm und
  wird bei einer Aktualisierung (`k-playbook upgrade`) ersetzt; dort wird nichts abgelegt und
  nichts geändert.
- **Das Mitgelieferte auf dem Hub.** Anweisungen, Review-Rezepte, Leitlinien, die Texte der
  Commands und das Handbuch liegen in der öffentlichen Collection `k-playbook` auf dem
  Kephalaion-Hub des Projekts; der Hub holt sie mit jedem Release neu. Gelesen wird das nur über
  `k-playbook:`-Verweise, nie direkt.
- **Das Projekt.** Im Projekt selbst liegen der Anker `.k-playbook/K-PLAYBOOK.yaml` — das
  Verzeichnis, das `.k-playbook/` enthält, ist die Projektwurzel —, eigene Commands und Skills
  unter `.k-playbook/commands/` und `.k-playbook/skills/` und das, was das Einrichten für die
  Assistenten anlegt (`AGENTS.md`, `CLAUDE.md`, die MCP-Einträge, die Links auf Commands und
  Skills). Alles Übrige, was das Projekt hervorbringt, liegt in seiner Collection in Kephalaion
  unter `keph:/`: die Projektdoku (`keph:/docs/`), Tasks und Workbench (`keph:/user/~/`),
  Befunde und Doku-Aufträge (`keph:/queue/`), die projekteigenen Anweisungen und Review-Rezepte
  (`keph:/k-playbook/`) und die Todos. Eine Aktualisierung berührt nichts davon.

Die vollständige Liste der Ablageorte steht in `k-playbook:/guidelines/betrieb.md`.

### Wie Anweisungen funktionieren

Anweisungen liegen in zwei Verzeichnissen, `always/` und `on-demand/`, und jedes hat zwei
Ebenen: die mitgelieferte auf dem Hub und die projekteigene in der Collection des Projekts unter
`keph:/k-playbook/instructions/always/` und `keph:/k-playbook/instructions/on-demand/`. Gelesen
werden beide Ebenen zusammen über `k-playbook:/instructions/…`; die Überlagerung rechnet
k-playbook.

**`always/` gilt in jeder Sitzung.**

- k-playbook baut alle Teile zu einem Text zusammen. Er kommt mit dem Briefing, eine Sitzung
  liest ihn nicht eigens; derselbe Text steht hinter `k-playbook:/instructions/always/`.
- Aufgenommen werden nur Dateien, deren Name mit einer Ziffer beginnt. Eine Datei ohne Ziffer
  vorn wird ausgelassen und als Problem gemeldet; Unterverzeichnisse und `README.md` zählen
  nicht.
- Sortiert wird numerisch nach der Ziffer am Anfang des Namens, mitgelieferte und projekteigene
  Teile gemeinsam: `20-` kommt vor `100-`. Bei gleicher Ziffer entscheidet der Name.
- Mitgelieferte Teile tragen Nummern unter 1000, projekteigene ab 1000; sie stehen damit hinter
  allem Mitgelieferten. Wer einen eigenen Teil gezielt davor setzen will, wählt eine kleinere
  Nummer.
- Ein projekteigener Teil mit demselben Dateinamen ersetzt den mitgelieferten vollständig. Die
  Ziffer gehört zum Namen: `100-projektwissen-zuerst.md` ersetzt genau diesen Teil,
  `150-projektwissen-zuerst.md` wäre ein neuer daneben.
- Ein leerer projekteigener Teil schaltet den mitgelieferten ab. Leer heißt: nur Leerzeilen und
  Zeilen, die mit `#` beginnen. Die Datei kann so ihren Grund nennen (`# Abgeschaltet: …`); auch
  eine Datei nur aus Überschriften schaltet also ab. Wer einen Teil ersetzen will, braucht
  mindestens eine Zeile Text.
- Das Frontmatter eines Teils wird vor dem Zusammenfügen entfernt.

**`on-demand/` gilt auf Abruf.**

- Jede Datei wird einzeln über ihren Namen gelesen, `k-playbook:/instructions/on-demand/<thema>.md`.
  Das Verzeichnis wird nicht zusammengebaut; der Verweis mit `/` am Ende ist ein Fehler.
- Die Dateien tragen keine Nummer und haben keine Reihenfolge. Mit `always/` teilen sie keinen
  Namensraum.
- Jede Datei trägt im Frontmatter `title` und `description`. Damit nennt das Briefing sie unter
  „Auf Abruf“, ohne ihren Inhalt zu laden. Geliefert wird die Datei unverändert, das Frontmatter
  eingeschlossen.
- Zuerst gilt die projekteigene Fassung gleichen Namens, sonst die mitgelieferte. Eine leere
  projekteigene Fassung schaltet die mitgelieferte ab; leer heißt dasselbe wie bei `always/`.
- Die Liste „Auf Abruf“ bringt keine Sitzung dazu, eine Anweisung zu lesen. Dafür steht in
  einem Always-Teil ein Satz, der sagt, wann sie zu lesen ist.

**Was gilt, zeigt das Briefing.** Jeder Eintrag nennt seine Herkunft: `dist` (mitgeliefert),
`local` (nur projekteigen), `override` (projekteigen ersetzt mitgeliefert) oder `disabled`
(abgeschaltet), bei einem projekteigenen Eintrag auch seinen `keph:`-Ort. Eine Anweisung liest
du deshalb immer über `k-playbook:`, nie selbst aus `keph:/k-playbook/`.

**Anlegen, ändern, abschalten.** Mitgeliefertes wird nie geändert. Eine projekteigene Anweisung
legst du in der Collection des Projekts an (Kephalaion `create`, später `write`; `collection`
gleich `kephalaion.address`, `name` etwa `k-playbook/instructions/always/1000-<thema>.md`). Sie
gilt für alle, die im Projekt arbeiten. Wer eine mitgelieferte Anweisung ersetzt, übernimmt den
ganzen Text; spätere Verbesserungen am Original erreichen die Kopie nicht mehr. Eine
projekteigene Anweisung schaltest du ab, indem du sie löschst.

**Includes.** Eine Zeile `include: <verweis>` am Zeilenanfang, mit genau einem Argument, wird
vor der Ausgabe durch den Rumpf des Ziels ersetzt, ohne dessen Frontmatter. Das gilt für jedes
Dokument, das k-playbook liest. Syntax, relative Pfade, Tiefe und Fehler stehen in
`k-playbook:/docs/handbook/verweisschema.md`, Abschnitt „Includes“.

### Eine `AGENTS.md` aufteilen

In der `AGENTS.md` bleibt nur der Anstoß-Block (Marke `<!-- k-playbook:anstoss -->`), der auf
das Briefing zeigt. `CLAUDE.md` ist nur der Include mit der Zeile `@AGENTS.md`. Das ist der
Zustand nach dem Aufteilen: Das Einrichten selbst hängt den Block an und lässt vorhandenen Text
einer `AGENTS.md` stehen. Was dort noch steht, gehört an eines dieser Ziele:

| Inhalt | Ziel |
|---|---|
| Projektregel, die in jeder Sitzung gilt | `keph:/k-playbook/instructions/always/<nummer ab 1000>-<thema>.md` |
| Regel für eine bestimmte Aufgabe | `keph:/k-playbook/instructions/on-demand/<thema>.md`, dazu ein Satz in einem Always-Teil, der sagt, wann sie zu lesen ist |
| gesichertes Wissen über das Projekt | `keph:/docs/manual/<thema>.md` |
| Vorgabe | `keph:/docs/guidelines/<thema>.md` |
| Schnipsel, dessen Ziel noch unklar ist | Befund in `keph:/queue/findings/`, nach `k-playbook:/instructions/on-demand/befunde.md` |

Aufgeteilt wird nur nach Rücksprache mit dem Nutzer, Abschnitt für Abschnitt. Ein Abschnitt
verschwindet aus der `AGENTS.md` erst, wenn er am Ziel geschrieben ist.

### Was in die Queue kommt

- **Befunde** nach `keph:/queue/findings/`, ein Dokument je Befund: was aus Analyse und
  Fehlersuche belegt ist, und Schnipsel, deren Ziel noch unklar ist. Ein Befund wartet darauf,
  in die Wissensablage übernommen zu werden. Die Form steht in
  `k-playbook:/instructions/on-demand/befunde.md`.
- **Doku-Aufträge** nach `keph:/queue/docs/`: Wer nach einer Codeänderung die erzeugte Doku
  (`keph:/docs/generated/code/`, `keph:/docs/generated/libs/`) nachgezogen haben will, schreibt
  sie nicht selbst, sondern reiht einen Auftrag ein; `/k-docs-code` bzw. `/k-docs-tools`
  arbeitet die Aufträge gebündelt ab. Die Form steht in
  `k-playbook:/instructions/on-demand/doku-auftraege.md`.

Ein Auftrag ist ein Merkzettel: Er sagt, dass und warum nachzuziehen ist, und enthält keine
Doku. Handgeschriebene Doku unter `keph:/docs/manual/` kommt nicht in die Queue; sie wird im
selben Arbeitsgang geändert.

### Weiterlesen

Das Handbuch von k-playbook, je Seite ein Verweis:

- `k-playbook:/docs/handbook/overview.md` — wofür k-playbook da ist: drei Arten von Arbeit, die
  gemeinsamen Mittel und die Projektdoku.
- `k-playbook:/docs/handbook/manual.md` — das Kernmodell in Kürze und der übliche Ablauf.
- `k-playbook:/docs/handbook/k-playbook-format.md` — `K-PLAYBOOK.yaml`, alle Ablageorte, die
  Überlagerung von Mitgeliefertem und Projekteigenem, der Anstoß-Block der `AGENTS.md`.
- `k-playbook:/docs/handbook/verweisschema.md` — die Präfixe, Includes und wie `k-playbook:`
  aufgelöst wird.
- `k-playbook:/docs/handbook/commands.md` — das Verzeichnis der Slash-Commands.
- `k-playbook:/docs/handbook/task-flow.md` — der Weg eines Tasks über `/k-task-create`,
  `/k-task-refine` und `/k-task-run`.
- `k-playbook:/docs/handbook/installation.md` — Installation, die drei Schritte des
  Einrichtens, das Paar `CLAUDE.md`/`AGENTS.md`, Aktualisieren.
- `k-playbook:/docs/handbook/faq.md` — kurze Antworten auf häufige Fragen.
