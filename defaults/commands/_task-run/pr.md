---
title: PR-Handoff
description: Modul für /k-task-run, vom Hauptkontext gelesen, wenn ein Task `PR required: true` trägt — Preflight, Push und Pull Request über gh, mit Dienst über k-playbook github run, PR-Text als Datei, und was ohne nutzbaren Weg ausgegeben wird.
---

# PR-Handoff

Dieses Modul liest der Hauptkontext von `/k-task-run` in 5f.1, wenn der `## Ausführungskontext`
des Tasks `PR required: true` trägt. Er hat `TASK_REF_DONE`, die Ausführungswurzel
(`EXEC_ROOT`), `Work branch` und `Base branch` aus 5a.1, `LOCK_TOKEN` und die Zeilen
„Validierung“ aus der letzten Rückgabe von Ausführung oder Behebung.

Der PR folgt erst nach der Stopp-Abfrage aus 5f, nachdem der Task abgeschlossen ist und ein
lokaler Commit existiert.

Zugang: Für Push und PR gilt der folgende Block. Das Ziel-Repo ist das der aufgelösten
Ausführungswurzel aus 5a.1 (`Target repo`), nicht zwingend das Arbeitsverzeichnis.

include: k-playbook:/commands/_shared/gh.md

Merke daraus `GH_SERVICE`, den Dienst für das Ziel-Repo; leer ohne Dienst. Push und PR
schreiben nach GitHub: Mit Dienst laufen beide über `k-playbook github run <GH_SERVICE> --write`,
ohne Dienst wie bisher über das aktive Konto. Vorher nennst du den Zugang, wie der Block es
verlangt.

Preflight:

1. In der aufgelösten Ausführungswurzel aus 5a.1 arbeiten.
2. Prüfen, dass der Branch `Work branch` ist, wenn einer angegeben war.
3. Prüfen, dass der Arbeitsbaum sauber ist (`git status --short`); sonst anhalten und melden,
   dass vor dem PR ein Commit nötig ist.
4. Prüfen, dass der Branch einen Upstream hat. Ohne Upstream vor dem Push fragen; nie still
   pushen.
5. `check` der Sperre, dann pushen. Mit Dienst:

   ```bash
   k-playbook github run <GH_SERVICE> --write -- git -C "<EXEC_ROOT>" push -u origin <Work branch>
   ```

   Ohne Dienst derselbe git-Aufruf ohne das vorangestellte `k-playbook github run … --`.

PR-Text:

- Den PR-Text als echtes mehrzeiliges Markdown in eine temporäre Datei schreiben, nicht als
  zitierten CLI-String mit `\n`.
- `/tmp/k-task-run-pr-body-<tasknummer-oder-branch-slug>.md`, sofern kein besserer
  temporärer Pfad schon da ist.
- Inhalt: kurze Zusammenfassung der Änderung; Befund-IDs oder der Verweis `TASK_REF_DONE`;
  die Zeilen „Validierung“ aus der letzten Rückgabe von Ausführung oder Behebung; bekannte
  Restrisiken oder Tests, die nicht laufen konnten.

PR anlegen mit `--body-file <datei>`, mit Dienst so:

```bash
k-playbook github run <GH_SERVICE> --write -- gh pr create \
  --base <Base branch> \
  --head <Work branch> \
  --title "<knapper Titel>" \
  --body-file /tmp/k-task-run-pr-body-<slug>.md
```

Kein `--body "...\n..."`: Bash übergibt in normalen doppelten Anführungszeichen ein
wörtliches Backslash-n. Ist ein Inline-Body unvermeidlich, ein Heredoc oder ANSI-C-Quoting
nehmen; `--body-file` bleibt der Weg. Ohne Dienst derselbe Aufruf ohne das vorangestellte
`k-playbook github run <GH_SERVICE> --write --`.

Gibt es nach dem Block oben keinen nutzbaren Weg — ein Dienst ohne Kanal `api`, ohne Dienst
`context.gh.ready` falsch — oder scheitert Push oder PR an der Anmeldung, gib den
genauen Befehl (mit Dienst in der Form mit `k-playbook github run`) und den Inhalt der
Body-Datei zum Nachmachen aus. Dazu kommt der Hinweis auf die Zugangsdatei samt dem Angebot,
beim Einrichten zu helfen („Die Zugangsdatei als Abhilfe“ im Block oben).
