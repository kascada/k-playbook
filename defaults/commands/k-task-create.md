# k-task-create

Legt aus dem laufenden Gespräch einen neuen Task an: ein Dokument in der Projekt-Collection
unter `keph:/user/~/tasks/`. Der Entwurf wird gezeigt und erst nach Bestätigung geschrieben.
Das Argument ist der Kurzname des Tasks; ohne Argument wird er aus dem Thema des Gesprächs
abgeleitet.

Produces:

- `keph:/user/<user>/tasks/<NNN>-<kurzname>.md` — der Task, angelegt mit
  `k_playbook_task_create`, erst nach Bestätigung.

Nichts sonst. Auf die Platte schreibt dieser Command nichts.

## Schritt 1 — Briefing und Form

include: _shared/erster-schritt.md

Verbindlich für Ablage, Name und Nummer, Kopf, Aufbau, Intent-Regel und Zuschnitt eines
Tasks ist `k-playbook:/instructions/on-demand/tasks.md`. Lies sie mit `k_playbook_doc_read`,
bevor du den Entwurf schreibst; dieser Text wiederholt sie nicht.

Steht `kephalaion.ok` nicht auf `true`, endet der Command hier, wie der Abschnitt
„Ohne Kephalaion“ dort sagt: kein Task auf der Platte, Meldung mit
`kephalaion.message`.

## Schritt 2 — Kurzname bestimmen

Ist ein Argument angegeben, ist es der Kurzname (klein, Wörter mit Bindestrichen). Sonst
leitest du ihn aus dem Thema des Gesprächs ab: 2–4 Wörter, klein, Bindestriche.

Die Nummer vergibt `k_playbook_task_create` beim Anlegen in Schritt 5; du zählst nichts und
gibst keine vor. Im Entwurf steht an ihrer Stelle `<NNN>`.

## Schritt 3 — Referenzen und Tools bestimmen

Sieh das Gespräch durch nach:

- Verweisen, die der Task braucht → `## Referenzen`, je mit einem Halbsatz, warum. Jeder
  Eintrag ist ein Verweis mit Präfix (`project:/`, `k-playbook:/`, `keph:/`); eine Datei im
  Projekt steht als `project:/<pfad>`, nicht als nackter Pfad. Lässt sich ein Präfix nicht
  eindeutig zuordnen, gilt `k-playbook:/instructions/on-demand/verweise-nachziehen.md`: fragen,
  nicht raten.
- Werkzeugen über Read, Write, Edit, Bash, Glob und Grep hinaus — MCP-Werkzeuge, besondere
  Rechte — → `## Tools`. Die Standardwerkzeuge stehen nicht dort; `/k-task-run` erlaubt sie
  immer.

## Schritt 4 — Entwurf schreiben und bestätigen lassen

Schreibe den Rumpf nach dem Abschnitt „Aufbau“ von
`k-playbook:/instructions/on-demand/tasks.md`: ein Satz zum Ergebnis, dann `## Intent`
(Intent-Regel dort, Verweise darin mit Präfix), `## Referenzen`, `## Tools`, `## Ziel`,
`## Kontext`, `## Zu bauen` mit `### Etappe N — Titel`, wenn die Arbeit in Etappen zerfällt.
Leere oder nicht zutreffende Abschnitte entfallen. Zuschnitt: im Zweifel eine Datei, wie dort
begründet. Frontmatter und die Überschrift `# Task <NNN> — <Titel>` schreibst du nicht; die
baut das Werkzeug, mit Frontmatter mit `title` in doppelten
Anführungszeichen (Regel in tasks.md unter „Kopf“). Ist der Task in einer Reihe nicht der letzte oder rein mechanisch, setzt du
`noIntent: true`, sonst nicht.

Rufe `k_playbook_task_create` mit `dryRun: true` auf (`projectDir` = `project.dir`,
`title`, `name` = Kurzname aus Schritt 2, `body`). Die Antwort trägt den Text, wie er
geschrieben würde (`rendered`), und die `warnings`. Meldet das Werkzeug `invalid_input`, stehen
die Verstöße gesammelt in der Meldung: arbeite sie ein und rufe erneut auf, bevor du dem
Nutzer etwas zeigst. Den Entwurf zeigst du dem Nutzer als `rendered`; Warnungen nennst du dazu.

Frage: „Passt das so, oder soll ich etwas anpassen?“ Warte auf Bestätigung oder Korrekturen,
bevor du etwas anlegst. Korrekturen arbeitest du in den Rumpf ein und wiederholst den
Trockenlauf.

## Schritt 5 — Anlegen

Nach der Bestätigung genau ein Aufruf: `k_playbook_task_create` ohne `dryRun`, mit denselben
`title`, `name`, `body` (und `noIntent`). Das Werkzeug vergibt die Nummer, baut Frontmatter
und Überschrift, prüft noch einmal und legt den Task mit fertigem Inhalt an; ein leeres
Zwischendokument gibt es nicht. Die Antwort nennt `ref` (den `keph:`-Verweis mit dem echten
User), `number`, `name` und `revision`. Bestätige in einer Zeile: „Task angelegt: `<ref>`“.

Fehlerfälle:

- `invalid_input`: Verstöße gegen den Aufbau; nichts wurde angelegt. Einarbeiten, Entwurf
  erneut zeigen.
- `kephalaion_unavailable`: wie Schritt 1 — nichts auf die Platte, Meldung mit
  `kephalaion.message`.
- `outcome_unknown`: nicht noch einmal anlegen, sondern mit `k_playbook_doc_list` (Sorte
  `tasks`, `name` = der Kurzname) nachsehen, ob der Task da ist; sonst den Nutzer fragen.

Folge-Command: **`/k-task-refine`** — härtet den angelegten Task vor der Ausführung im
Critic/Editor-Dialog; ohne Argument nimmt er die eigenen offenen Tasks, die noch kein
Review-Log tragen — also auch diesen —, mit dem Verweis genau diesen.
