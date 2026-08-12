# Task-Flow

Der Task-Flow ist der Standardweg für geplante Arbeit, die nicht direkt in einem kurzen Chat-Schritt erledigt werden soll.

## Standardablauf

```text
/k-task-create
/k-review-loop
/k-run
```

Tasks können direkt aus dem Gespräch entstehen oder von `/k-remediation` erzeugt werden. In beiden Fällen gilt: erst Task-Dateien prüfen, dann ausführen.

## /k-task-create

`/k-task-create [short-name]` erzeugt eine strukturierte Task-Datei unter `k-playbook-local/tasks/`.

Der Command:

- leitet den Ort aus der Lage der `K-PLAYBOOK.yaml` ab.
- bestimmt die nächste freie Nummer aus offenen Tasks und `done/`.
- erzeugt einen Dateinamen wie `014-audiosocket-server.md`.
- nimmt relevante Referenzen und besondere Tools in die Task-Datei auf.
- zeigt den Entwurf zuerst und speichert erst nach Bestätigung.

Tasks sollen so geschrieben sein, dass `/k-run` sie ohne weiteren Chatkontext ausführen kann.

Umfangreiche Arbeit wird nicht auf viele kleine Dateien verteilt, sondern in einer Datei unter `## Zu bauen` als `### Etappe N — Titel` gegliedert. Geteilt wird nur, was sachlich auseinanderfällt: unterschiedlicher Kontext, einzeln sinnvoll, einzeln verifizierbar. Der Grund, allein wegen des Umfangs zu teilen, entfällt durch die Fortschrittsverfolgung in `/k-run`.

## /k-review-loop

`/k-review-loop [path]` prüft Task- oder Instruktionsdateien vor der Ausführung.

Der Command nutzt einen strukturierten Critic/Editor-Dialog:

- Critic und Editor sind read-only.
- Der Moderator ist der einzige Writer.
- Tatsächlicher Dateistand gewinnt nach jeder Änderung.
- Akzeptierte Edits und Entscheidungen werden im Review-Log festgehalten.
- Am Ende wird gegen den angegebenen `## Intent` geprüft, falls vorhanden.

Jede geprüfte Datei bekommt ein `## Review-Log`, auch die, an der nichts zu ändern war — dort mit dem Vermerk „keine Änderungen". Das Log ist der einzige Beleg, dass ein Review stattgefunden hat; `/k-run` erkennt daran, ob ein Task gegengelesen wurde.

Ohne Argument prüft der Command die offenen Task-Dateien unter `k-playbook-local/tasks/`.

## /k-run

`/k-run [file-or-directory]` führt Task-Dateien sequenziell aus.

Der Command:

- nutzt ohne Argument `k-playbook-local/tasks/`.
- sortiert Tasks nach numerischem Prefix.
- führt Tasks nie parallel aus.
- fragt nach, wenn eine Task-Datei kein Review-Log trägt, also nie durch `/k-review-loop` gegangen ist.
- klärt offene Fragen vor Delegation an Subagenten.
- hängt eine Ausführungsnotiz an.
- verschiebt erfolgreich abgeschlossene Tasks nach `done/`.
- lässt abgebrochene oder teilweise ausgeführte Tasks offen.

Enthält eine Task-Datei `### Etappe`-Überschriften, führt `/k-run` darin eine `## Fortschritt`-Tabelle. Der Subagent trägt jede Etappe direkt nach ihrem Abschluss ein, nicht erst am Ende — so überlebt der Stand einen harten Abbruch. Ein späterer `/k-run` liest die Tabelle und bietet an, bei der ersten offenen Etappe fortzusetzen. Die Tabelle ist die einzige Quelle für den Stand; aus dem Code oder aus `git log` wird nichts abgeleitet.

Wenn eine Task-Datei `## Ausführungskontext` enthält, wertet `/k-run` daraus unter anderem `Target repo`, `Base branch`, `Work branch` und `PR required` aus. Dann gehören Branch-/Dirty-Worktree-Preflight und ggf. PR-Handoff zum Ablauf.

## Remediation-Tasks

Von `/k-remediation` erzeugte Tasks sind normale Task-Flow-Eingaben. Besonders wichtig sind dabei:

- Findings-IDs und Quellen müssen in der Task stehen.
- Branch-/PR-Anforderungen aus der Remediation-Policy müssen im Ausführungskontext stehen.
- Vor Umsetzung läuft `/k-review-loop`.
- Umsetzung läuft danach über `/k-run`.
