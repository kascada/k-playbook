---
title: GitHub über run
---

## GitHub über run

Netzbefehle von git (clone, fetch, pull, push, ls-remote) und jedes `gh` gegen das Repo eines
Dienstes laufen über `k-playbook github run <dienst> -- …`, was nach GitHub schreibt, mit
`--write`. Den Dienst wählst du nach dem Ziel-Repo des Befehls: Für das Repo des
Arbeitsverzeichnisses nennt ihn die Briefing-Zeile `GitHub: dieses Repo = Dienst … — run: …`
(zusätzlich das Feld `context.github.projectServices`), für ein anderes Repo
`k-playbook github show`. Gibt es keinen Dienst oder keine Zugangsdatei, arbeitest du wie bisher.

Schlägt `run` fehl, weichst du nicht still auf nacktes `gh` oder `git` aus und wechselst den
Zugang nicht ungefragt. Einen Token-Wert gibst du nie aus.

Die übrigen Ausnahmen, das Fehlerbild und die Verbote stehen in
`k-playbook:/instructions/on-demand/github.md`; lies es vor dem ersten GitHub-Zugriff der Sitzung.
