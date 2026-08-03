# Task-Flow

Der Task-Flow ist der Standardweg fuer geplante Arbeit, die nicht direkt in einem kurzen Chat-Schritt erledigt werden soll.

## Standardablauf

```text
/k-task-create
/k-review-loop
/k-run
```

Tasks koennen direkt aus dem Gespraech entstehen oder von `/k-remediation` erzeugt werden. In beiden Faellen gilt: erst Task-Dateien pruefen, dann ausfuehren.

## /k-task-create

`/k-task-create [short-name]` erzeugt eine strukturierte Task-Datei unter `paths.tasks`.

Der Command:

- liest `K-PLAYBOOK.yaml` und nutzt `paths.tasks`.
- bestimmt die naechste freie Nummer aus offenen Tasks und `done/`.
- erzeugt einen Dateinamen wie `014-audiosocket-server.md`.
- nimmt relevante Referenzen und besondere Tools in die Task-Datei auf.
- zeigt den Entwurf zuerst und speichert erst nach Bestaetigung.

Tasks sollen so geschrieben sein, dass `/k-run` sie ohne weiteren Chatkontext ausfuehren kann.

## /k-review-loop

`/k-review-loop [path]` prueft Task- oder Instruktionsdateien vor der Ausfuehrung.

Der Command nutzt einen strukturierten Critic/Editor-Dialog:

- Critic und Editor sind read-only.
- Der Moderator ist der einzige Writer.
- Tatsaechlicher Dateistand gewinnt nach jeder Aenderung.
- Akzeptierte Edits und Entscheidungen werden im Review-Log festgehalten.
- Am Ende wird gegen den angegebenen `## Intent` geprueft, falls vorhanden.

Ohne Argument prueft der Command die offenen Task-Dateien unter `paths.tasks`.

## /k-run

`/k-run [file-or-directory]` fuehrt Task-Dateien sequenziell aus.

Der Command:

- nutzt ohne Argument `paths.tasks`.
- sortiert Tasks nach numerischem Prefix.
- fuehrt Tasks nie parallel aus.
- klaert offene Fragen vor Delegation an Subagenten.
- haengt eine Ausfuehrungsnotiz an.
- verschiebt erfolgreich abgeschlossene Tasks nach `done/`.
- laesst abgebrochene oder teilweise ausgefuehrte Tasks offen.

Wenn eine Task-Datei `## Ausfuehrungskontext` enthaelt, wertet `/k-run` daraus unter anderem `Target repo`, `Base branch`, `Work branch` und `PR required` aus. Dann gehoeren Branch-/Dirty-Worktree-Preflight und ggf. PR-Handoff zum Ablauf.

## Remediation-Tasks

Von `/k-remediation` erzeugte Tasks sind normale Task-Flow-Eingaben. Besonders wichtig sind dabei:

- Findings-IDs und Quellen muessen in der Task stehen.
- Branch-/PR-Anforderungen aus der Remediation-Policy muessen im Ausfuehrungskontext stehen.
- Vor Umsetzung laeuft `/k-review-loop`.
- Umsetzung laeuft danach ueber `/k-run`.
