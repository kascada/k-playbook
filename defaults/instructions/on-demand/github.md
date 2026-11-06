---
title: GitHub über run
description: Wie eine Sitzung GitHub über k-playbook github run erreicht — den Dienst nach dem Ziel-Repo bestimmen, gh.status disabled, --write und das Nennen vor dem Schreiben, das Fehlerbild samt Exit-Codes 125, 126 und 127, die fünf Ausnahmen, in denen es beim Bisherigen bleibt, die Verbote rund um Token und Anmeldung und Beispiele.
---

## GitHub über run

Die immer geltende Anweisung „GitHub über run“ sagt die Regel: Netzbefehle von git (clone,
fetch, pull, push, ls-remote) und jedes `gh` gegen das Repo eines Dienstes laufen über
`k-playbook github run <dienst> -- …`. Hier stehen die Einzelheiten. `run` ist ein Befehl für
die Shell, kein MCP-Werkzeug; eine Sitzung mit Shell ruft ihn dort auf.

### Den Dienst bestimmen

Maßgeblich ist das Repo, auf das der Befehl zielt, nicht das Arbeitsverzeichnis.

- **Repo des Arbeitsverzeichnisses:** Die Briefing-Zeile `GitHub: dieses Repo = Dienst … —
  run: …` nennt den Dienst, dazu das Feld `context.github.projectServices`. Beides zeigt den
  Stand beim Laden des Briefings.
- **Ein anderes Repo** — `gh … --repo <owner>/<repo>`, `gh api` gegen eine andere Org,
  `git clone` eines fremden Repos: `k-playbook github show` nennt alle Dienste, je mit ihrem
  `repo`, und den aktuellen Stand.

Danach:

- genau ein Dienst passt: diesen nehmen;
- mehrere passen: den Nutzer fragen, welcher gilt;
- keiner passt (keine Zugangsdatei, kein GitHub-Remote, kein Dienst mit diesem `repo`): wie
  bisher, also nach `context.gh` bzw. nach der eigenen Regel des Commands.

### gh gesperrt

Steht `context.gh.status` auf `disabled`, nutzt das Projekt `gh` nicht — auch nicht unter `run`.
Netzbefehle von git über `run` bleiben davon unberührt: `gh.status` ist die Entscheidung über
die GitHub CLI, nicht über git.

### Schreiben

- Was nach GitHub schreibt, läuft mit `--write`: push, einen Pull Request anlegen,
  kommentieren, freigeben, mergen, ein Release anlegen. `--write` überspringt den
  Nur-Lese-Zugang `public`.
- Nach GitHub schreibst du nur auf Auftrag.
- Vor dem Schreiben nennst du Dienst und Zugang: aus der Zeile, die `run` vor dem Start auf
  stderr ausgibt (`github run <dienst>: git über …, api über …`), oder vorab aus `selected.write`
  in `k-playbook github show --json`, der Wahl von `run --write`.
- `--quiet` nutzt du nicht: Die Zeile zeigt, unter welchem Zugang der Befehl lief.

### Wenn run scheitert

`run` startet den Befehl genau einmal und wählt keinen anderen Zugang. Der Exit-Code sagt, wer
gescheitert ist:

- `125`: `run` selbst — Aufruf, unbekannter Dienst, keine Datei, eine Datei mit Fehlern, kein
  nutzbarer Zugang, ein Token nicht lesbar. Du meldest die Meldung von `run`. Fehlt die Datei,
  nennt sie `k-playbook github init`; das rufst du nur, wenn der Nutzer es will.
- `126`: der Befehl ist nicht ausführbar; `127`: der Befehl wurde nicht gefunden. Beides kommt
  ebenfalls von `run`.
- Jeder andere Code kommt vom Befehl.

Deutet der Fehler auf die Anmeldung, prüft `k_playbook_github_test` (ersatzweise
`k-playbook github test <dienst>`) die Zugänge des Dienstes. Eine andere Wahl mit `--use <typ>`
oder `--skip <typ>` schlägst du vor; du nimmst sie nicht selbst. Nacktes `gh` oder `git` als
Ersatz gibt es nicht.

### Ausnahmen

In diesen Fällen arbeitest du wie bisher, ohne `run`:

- **Kein Dienst** bzw. keine Zugangsdatei (siehe „Den Dienst bestimmen“).
- **Lokale git-Befehle ohne Netz**, etwa status, log, diff, commit, branch.
- **Commands mit eigener gh-Regel:** `/k-pr-review`, der PR-Schritt in `/k-task-run` und das
  Review `k-playbook:/reviews/review-dependabot-alerts.md` folgen bis zu ihrer Umstellung ihrer
  eigenen Regel.
  <!-- NACHZIEHEN: Die Commands mit eigener gh-Regel werden auf run umgestellt; danach entfällt diese Ausnahme und mit ihr context.gh. -->
- **Make-Ziele und Skripte:** Die, die `run` schon selbst rufen, laufen ohne weitere Hülle.
  Die übrigen wickelst du nicht in `run` ein; sie laufen wie bisher.
- **Eine ausdrückliche Ansage des Nutzers**, etwa einen bestimmten Zugang oder nacktes `gh` zu
  nehmen.

### Verbote

- Du liest keine Token-Dateien und gibst keinen Token-Wert aus.
- Du setzt `GH_TOKEN` oder `GITHUB_TOKEN` nicht selbst; das tut `run` für den Befehl.
- Du schreibst die Zugangsdatei `github.yaml` nicht. `k-playbook github init` rufst du nur auf
  Wunsch des Nutzers.
- Du rufst `gh auth login`, `gh auth switch` und `gh auth token` nicht.

### Beispiele

```text
k-playbook github run <dienst> -- git fetch origin
k-playbook github run <dienst> -- gh pr list
k-playbook github run <dienst> --write -- git push origin feature/x
k-playbook github run <dienst> --write -- gh pr create --base main --draft --fill
k-playbook github run <dienst> --write --skip ssh -- git push origin feature/x
```

Das letzte Beispiel ist eine andere Wahl, die du vorschlägst, nicht selbst triffst.
