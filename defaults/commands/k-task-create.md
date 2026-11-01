# k-task-create

Legt aus dem laufenden Gespräch einen neuen Task an: ein Dokument in der Projekt-Collection
unter `keph:/user/~/tasks/`. Der Entwurf wird gezeigt und erst nach Bestätigung geschrieben.
Das Argument ist der Kurzname des Tasks; ohne Argument wird er aus dem Thema des Gesprächs
abgeleitet.

Produces:

- `keph:/user/<user>/tasks/<NNN>-<kurzname>.md` — der Task, angelegt mit
  `k_playbook_doc_create` und geschrieben mit Kephalaion `write`, erst nach Bestätigung.

Nichts sonst. Auf die Platte schreibt dieser Command nichts.

## Schritt 1 — Briefing und Form

include: _shared/erster-schritt.md

Verbindlich für Ablage, Name und Nummer, Kopf, Aufbau, Intent-Regel und Zuschnitt eines
Tasks ist `k-playbook:/instructions/on-demand/tasks.md`. Lies sie mit `k_playbook_doc_read`,
bevor du den Entwurf schreibst; dieser Text wiederholt sie nicht.

Steht `context.kephalaion.ok` nicht auf `true`, endet der Command hier, wie der Abschnitt
„Ohne Kephalaion“ dort sagt: kein Task auf der Platte, Meldung mit
`context.kephalaion.message`.

## Schritt 2 — Kurzname bestimmen

Ist ein Argument angegeben, ist es der Kurzname (klein, Wörter mit Bindestrichen). Sonst
leitest du ihn aus dem Thema des Gesprächs ab: 2–4 Wörter, klein, Bindestriche.

Die Nummer vergibt k-playbook beim Anlegen in Schritt 5; du zählst nichts und gibst keine
vor. Im Entwurf steht an ihrer Stelle `<NNN>`.

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

Schreibe den Entwurf nach dem Abschnitt „Aufbau“ von
`k-playbook:/instructions/on-demand/tasks.md`: Frontmatter mit `title`, Überschrift
`# Task <NNN> — <Titel>`, ein Satz zum Ergebnis, dann `## Intent` (Intent-Regel dort, Verweise
darin mit Präfix), `## Referenzen`, `## Tools`, `## Ziel`, `## Kontext`, `## Zu bauen` mit
`### Etappe N — Titel`, wenn die Arbeit in Etappen zerfällt. Leere oder nicht zutreffende
Abschnitte entfallen. Zuschnitt: im Zweifel eine Datei, wie dort begründet.

Zeige den Entwurf dem Nutzer und frage: „Passt das so, oder soll ich etwas anpassen?“ Warte
auf Bestätigung oder Korrekturen, bevor du etwas anlegst. Korrekturen arbeitest du in den
Entwurf ein und zeigst ihn erneut.

## Schritt 5 — Anlegen und schreiben

Nach der Bestätigung, in dieser Reihenfolge:

1. `k_playbook_doc_create` mit `kind: tasks` und dem Kurzname aus Schritt 2, `projectDir` =
   `context.project.dir`. Die Antwort nennt `ref` (den `keph:`-Verweis mit dem echten User),
   `number` und `name`.
2. Setze `<NNN>` in der Überschrift auf `number`, dreistellig.
3. Schreibe den ganzen Entwurf samt Frontmatter mit Kephalaion `write`: `collection` =
   `context.kephalaion.address`, `name` = der Pfad des Verweises ohne führenden `/`. Die
   Antwort nennt Revision und Größe; erst damit ist der Task angelegt.
4. Bestätige in einer Zeile: „Task angelegt: `<ref>`“.

Fehlerfälle:

- `kephalaion_unavailable` bei `k_playbook_doc_create`: wie Schritt 1 — nichts auf die
  Platte, Meldung mit `context.kephalaion.message`.
- `outcome_unknown`: nicht noch einmal anlegen, sondern mit `k_playbook_doc_list` (Sorte
  `tasks`) nachsehen, ob das Dokument da ist; dann mit `write` weitermachen oder den Nutzer
  fragen.
- `write` schlägt fehl: Das leere Dokument bleibt liegen; sag das, nenne den Verweis und
  wiederhole `write`, bevor du etwas anderes tust. Ein zweites `k_playbook_doc_create` legte
  einen zweiten Task an.

Folge-Command: **`/k-task-refine`** — härtet den angelegten Task vor der Ausführung im
Critic/Editor-Dialog; ohne Argument nimmt er die eigenen offenen Tasks, mit dem Verweis
genau diesen.
