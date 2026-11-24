---
title: GitHub über run
description: Wie eine Sitzung GitHub über k-playbook github run erreicht — den Dienst nach dem Ziel-Repo bestimmen, --write und das Nennen vor dem Schreiben, das Fehlerbild samt Exit-Codes 125, 126 und 127, eine Ablehnung durch Nutzer oder Client, die Berechtigungen, die k-playbook in einem Projekt mit Dienst für Claude Code und OpenCode schreibt, samt ihren Lücken und dem Umgang mit Zielen ohne Dienst und einer ausdrücklichen Ansage, der Weg ohne Dienst mit der Zugangsdatei als Abhilfe, die vier Ausnahmen, in denen es beim Bisherigen bleibt, die Verbote rund um Token und Anmeldung und Beispiele.
---

## GitHub über run

Die immer geltende Anweisung „GitHub über run“ sagt die Regel: Netzbefehle von git (clone,
fetch, pull, push, ls-remote) und jedes `gh` gegen das Repo eines Dienstes laufen über
`k-playbook github run <dienst> -- …`. Hier stehen die Einzelheiten. `run` ist ein Befehl für
die Shell, kein MCP-Werkzeug; eine Sitzung mit Shell ruft ihn dort auf.

### Den Dienst bestimmen

Maßgeblich ist das Repo, auf das der Befehl zielt, nicht das Arbeitsverzeichnis.

- **Repo des Arbeitsverzeichnisses:** Die Briefing-Zeile `GitHub: dieses Repo = Dienst … —
  run: …` nennt den Dienst, dazu das Feld `github.projectServices`. Beides zeigt den
  Stand beim Laden des Briefings und gilt nur für das Repo an der Projektwurzel. Ein eigenes
  Repo in einem Unterverzeichnis (etwa ein eingebetteter Clone) ist ein anderes Repo.
- **Ein anderes Repo** — `gh … --repo <owner>/<repo>`, `gh api` gegen eine andere Org,
  `git clone` eines fremden Repos: `k-playbook github show` nennt alle Dienste, je mit ihrem
  `repo`, und den aktuellen Stand.

Danach:

- genau ein Dienst passt: diesen nehmen;
- mehrere passen: den Nutzer fragen, welcher gilt;
- keiner passt (keine Zugangsdatei, kein GitHub-Remote, kein Dienst mit diesem `repo`): wie
  bisher, also nach `environment.gh` (Abschnitt „Ohne Dienst“).

`environment.gh` und `environment.github` stehen nicht im Briefing. Wer eines der Felder
braucht, holt an dieser Stelle den Abruf `environment` — `k_playbook_briefing` mit
`part: "environment"`, ersatzweise `k-playbook briefing --part environment`.

Commands und Reviews, die GitHub erreichen, entscheiden den Zugang an einer Stelle:
`k-playbook:/commands/_shared/gh.md`. Sie binden sie ein oder lesen sie und wiederholen sie
nicht.

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

Lehnen der Nutzer oder die Berechtigungen des Clients den Aufruf von `run` ab, ist das kein
Fehler von `run`, von git oder von GitHub. Du nimmst keinen anderen Aufrufweg, der die
Ablehnung umgeht — weder nacktes `gh` oder `git` noch ein Skript —, sondern nennst den
abgelehnten Befehl und klärst die Freigabe mit dem Nutzer.

Lokale git-Befehle ohne Netz (Abschnitt „Ausnahmen“) wickelst du nicht in `run`: Sie
erreichen GitHub nicht, und eine Freigabe für `run` würde die Regeln des Clients für git
umgehen.

### Berechtigungen der Clients

In einem Projekt mit Dienst schreibt k-playbook die Berechtigungen von Claude Code
(`.claude/settings.local.json`) und OpenCode (`.opencode/opencode.jsonc`), beim Einrichten,
mit `k-playbook upgrade` und beim Start des Programms; beide Dateien bleiben unversioniert.
Gesperrt sind nacktes `gh` und die Netzbefehle von git (clone, fetch, pull, push, ls-remote,
remote update, auch mit `git -C <pfad>`). Ohne Rückfrage laufen
`k-playbook github run <dienst> -- gh …` und `-- git fetch|pull|clone|ls-remote|remote update …`.
Eine Rückfrage lösen `--write` und schreibende gh-Unterbefehle hinter `--` aus (`pr merge`,
`pr create`, `pr comment`, `pr review`, `pr close`, `release`, `repo`, `api` mit `-X` oder
`--method`); jede andere Zeile hinter `run` fragt ebenfalls, in Claude Code mangels Freigabe.
Fällt der Dienst weg, entfernt k-playbook seine Einträge wieder.

**Ziele ohne Dienst und eine ausdrückliche Ansage.** Die Sperre hängt am Projekt, der Dienst am
Ziel-Repo. In einem Projekt mit Dienst ist nacktes `gh` deshalb auch dort gesperrt, wo es nach
„Den Dienst bestimmen“ erlaubt wäre: bei einem Ziel ohne Dienst (`gh … --repo <fremdes repo>`,
`git clone` eines fremden Repos, ein eingebettetes Repo ohne Dienst) und wenn der Nutzer
ausdrücklich nacktes `gh` verlangt. Dann sagst du, dass die Berechtigungen des Clients den
Befehl sperren, und nennst ihn wörtlich; der Nutzer führt ihn selbst aus oder gibt ihn im
Einzelfall von Hand frei. Die Berechtigungen änderst du nicht, und du nimmst keinen Umweg.

**Lücken, die bleiben.** Die Muster sperren nicht alles. Bekannt und hingenommen:

- Hinter `run` fragt Schreibendes nur, wenn `--write` dasteht oder der Unterbefehl in der Liste
  oben steht. `-- git push` fragt trotzdem, weil es keine Freigabe hat.
- `*` trifft auch Text in Anführungszeichen: `run <dienst> -- git commit -m "a -- git fetch"`
  läuft ohne Rückfrage durch.
- In OpenCode ist eine Zeile ohne passende Regel erlaubt. Umwege wie `env gh`, `bash -c "gh …"`,
  ein absoluter Pfad zu `gh` oder ein Teil einer Kette nach `&&`, `;` oder `|` laufen dort
  durch, soweit keine Regel des Projekts sie trifft. In Claude Code fragt eine Zeile ohne
  Regel, außer einem Befehl, den Claude Code als nur lesend selbst erlaubt (etwa `ls`), auch
  als Teil einer Kette.
- `opencode run --auto` macht jede Rückfrage zur Freigabe; dort hält nur die Sperre.

Eine Lücke ist kein Weg: Die Regel „GitHub über run“ gilt auch dort, wo der Client sie nicht
durchsetzt.

### Ohne Dienst

Ohne Dienst gilt der bisherige Weg über das aktive Konto von `gh`: `environment.gh.ready` und das
aktive Konto aus `environment.gh` zählen nur hier. Geht dieser Weg nicht — `environment.gh.ready` ist falsch,
oder ein Aufruf scheitert an der Anmeldung (etwa HTTP 401) —, lautet die Meldung: Lege die
Zugangsdatei an. Dazu bietest du an, beim Einrichten zu helfen, statt nur auf die Anmeldung
von `gh` zu verweisen:

- `k-playbook github init` rufst du nur, wenn der Nutzer es will.
- Den Eintrag für den Dienst schlägst du vor: `repo` aus `environment.github.repo` bzw. aus dem
  Ziel-Repo, die Zugangsart nach Rückfrage. `gh` braucht einen Zugang vom Typ `gh` oder
  `token`; `ssh` und `public` bedienen nur git.
- Der Nutzer trägt den Eintrag ein; du schreibst die Datei nicht (Abschnitt „Verbote“).
- Danach prüfen `k-playbook github check` und `k-playbook github test <dienst>` (ersatzweise
  `k_playbook_github_test`) den Eintrag.

Dasselbe Angebot gilt, wenn ein Dienst keinen Zugang vom Typ `gh` oder `token` hat und ein
`gh`-Aufruf ansteht. `run` entfernt dann nur `GH_TOKEN` und `GITHUB_TOKEN`, und `gh` nähme still
das aktive Login des Rechners; deshalb rufst du in diesem Fall kein `gh`
(`k-playbook:/commands/_shared/gh.md`, „Dienst ohne Kanal `api`“).

### Ausnahmen

In diesen Fällen arbeitest du wie bisher, ohne `run`:

- **Kein Dienst** bzw. keine Zugangsdatei (siehe „Den Dienst bestimmen“).
- **Lokale git-Befehle ohne Netz**, etwa status, log, diff, commit, branch.
- **Make-Ziele und Skripte:** Die, die `run` schon selbst rufen, laufen ohne weitere Hülle.
  Die übrigen wickelst du nicht in `run` ein; sie laufen wie bisher.
- **Eine ausdrückliche Ansage des Nutzers**, etwa einen bestimmten Zugang oder nacktes `gh` zu
  nehmen. In einem Projekt mit Dienst sperren die Berechtigungen des Clients nacktes `gh`
  trotzdem (Abschnitt „Berechtigungen der Clients“).

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
