---
title: GitHub-Zugang vor dem ersten Aufruf
description: Welcher Zugang für GitHub gilt, bevor ein Command oder Review gh oder einen Netzbefehl von git ruft — Dienst nach dem Ziel-Repo, context.gh.status in beiden Zweigen, mit Dienst k-playbook github run samt --write, ein Dienst ohne Kanal api, ohne Dienst context.gh.ready und context.gh.account, die Zugangsdatei als Abhilfe samt Hilfe beim Einrichten und das Nennen vor dem Schreiben.
---

Diese Stelle entscheidet, welcher Zugang für GitHub gilt. Wer sie einbindet oder liest,
wiederholt sie nicht. Fehlerbild, Exit-Codes und Verbote stehen in
`k-playbook:/instructions/on-demand/github.md` und gelten hier unverändert.

**Den Dienst bestimmen.** Maßgeblich ist das Repo, auf das der Befehl zielt, nicht das
Arbeitsverzeichnis. `context.github.projectServices` und die Briefing-Zeile `GitHub:` gelten
nur für das Repo des Arbeitsverzeichnisses. Für ein anderes Repo — etwa `Target repo` in
`/k-task-run` oder eine PR-URL in `/k-pr-review` — nennt `k-playbook github show` alle Dienste
mit ihrem `repo`. Passt genau ein Dienst, nimmst du ihn; passen mehrere, fragst du, welcher
gilt; passt keiner, gilt der Zweig „Ohne Dienst“. Einzelheiten stehen im Abschnitt „Den Dienst
bestimmen“ in `k-playbook:/instructions/on-demand/github.md`.

**Zuerst `context.gh.status`, in beiden Zweigen.** Steht er auf `disabled`, hat sich das
Projekt gegen `gh` entschieden: kein `gh`, auch nicht unter `run`; halte an und melde es. Steht
er auf `unknown`, hat niemand entschieden: halte an und verweise auf `/k-gui`. Netzbefehle von
git über `run` betrifft das nicht — `gh.status` ist die Entscheidung über die GitHub CLI, nicht
über git.

**Mit Dienst.** Jeder Netzbefehl gegen das Repo — git (clone, fetch, pull, push, ls-remote)
und jedes `gh` — läuft als `k-playbook github run <dienst> -- …`; was nach GitHub schreibt
(push, Pull Request anlegen, freigeben, kommentieren, mergen), als
`k-playbook github run <dienst> --write -- …`. Welcher Zugang gilt, sagt `selected` des
Dienstes (`context.github.services.<dienst>.selected`, für ein anderes Repo
`k-playbook github show --json`); `context.gh.ready` und `context.gh.account` zählen in diesem
Zweig nicht.

**Dienst ohne Kanal `api`.** Ist `selected.read.api` — bzw. mit `--write` `selected.write.api`
— `null`, hat der Dienst keinen Zugang vom Typ `gh` oder `token`, etwa nur `ssh` oder `public`.
Dann rufst du kein `gh`. `run` entfernt in diesem Fall nur `GH_TOKEN` und `GITHUB_TOKEN`, und
`gh` nimmt still das aktive Login des Rechners: Am 2026-11-07 meldete `gh api user` unter `run`
mit einem Dienst nur aus `public` und mit einem nur aus `ssh` jeweils das aktive Konto. Melde
stattdessen, dass der Dienst für `gh` einen Zugang vom Typ `gh` oder `token` braucht, und biete
Hilfe an wie unter „Die Zugangsdatei als Abhilfe“. git über `run` bleibt davon unberührt.

**Ohne Dienst.** Es gilt der bisherige Weg über das aktive Konto von `gh`. Ist
`context.gh.ready` falsch, halte an, statt `gh` zu rufen, und nenne, was fehlt: `gh` ist nicht
installiert, oder es ist kein Konto hinterlegt. Installiere nie selbst und melde dich nie
selbst an. Für die fehlende Anmeldung ist der Hinweis die Zugangsdatei (nächster Absatz);
`gh auth login --hostname github.com` im Terminal des Nutzers bleibt möglich, ist aber nicht
der einzige Weg, den du nennst.

**Die Zugangsdatei als Abhilfe.** Geht der alte Weg nicht — `context.gh.ready` ist falsch, oder
ein Aufruf scheitert an der Anmeldung (etwa HTTP 401, „authentication required“) —, lautet die
Meldung: Lege die Zugangsdatei an. Ein Dienst für das Repo in der Datei aus
`context.github.file` legt fest, welcher Zugang gilt, unabhängig vom aktiven Konto. Biete an,
beim Einrichten zu helfen:

- `k-playbook github init` legt die Datei aus der Vorlage an; du rufst es nur, wenn der Nutzer
  es will.
- Du schlägst den Eintrag für den Dienst vor: `repo` aus `context.github.repo` bzw. aus dem
  Ziel-Repo, die Zugangsart (`gh` mit `login`, `token` mit `file`, `ssh`, `public`) nach
  Rückfrage. `gh` braucht einen Zugang vom Typ `gh` oder `token`.
- Der Nutzer trägt ihn ein; du schreibst `github.yaml` nicht selbst.
- Danach prüfen `k-playbook github check` und `k-playbook github test <dienst>` (ersatzweise
  `k_playbook_github_test`) den Eintrag.

**Vor dem Schreiben nennen.** Freigeben, mergen, kommentieren, pushen und einen Pull Request
anlegen sind nach außen sichtbar. Nenne vorher in einer Zeile, unter welchem Zugang es
geschieht. Mit Dienst sind das Dienst und Zugang — aus `selected.write` in
`k-playbook github show --json` oder aus der Zeile, die `run` vor dem Start auf stderr ausgibt.
Ohne Dienst ist es `context.gh.account`: Das aktive Konto gilt rechnerweit und kann seit dem
letzten Mal in einem anderen Projekt umgeschaltet worden sein. Ist `context.gh.account` leer,
läuft die Anmeldung über `GH_TOKEN`/`GITHUB_TOKEN`; sag das, statt einen Namen zu erfinden.
