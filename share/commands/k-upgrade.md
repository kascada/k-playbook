---
description: Zieht Kephalaion, Programm und Projekt nach — zuerst Kephalaion über kephalaion upgrade --auto, soweit Kephalaion das selbst für gefahrlos hält, dann das Programm, wenn es älter ist als das neueste Release, dann bringt es AGENTS.md, CLAUDE.md, Verlinkung und MCP-Einträge auf den Stand und führt danach die Umstellung durch, die eine Beurteilung braucht (3.8) — er löscht ohne Rückfrage einen alten Clone k-playbook/ ganz — ein echtes Verzeichnis mit eigenem .git-Verzeichnis, sonst bleibt es liegen und wird gemeldet — und nennt, was darin nicht vom Repo kam, und streicht aus jeder .gitignore vom Projektverzeichnis bis zur Repo-Wurzel die Zeilen, die k-playbook/ ausnehmen, sobald das Verzeichnis weg ist; mit einem Programm vor v0.16.39 wartet das auf den Binärwechsel.
# model: github-copilot/gpt-5.5
allowed-tools: [Bash, Read, Edit, Grep, Glob, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read]
---

# k-upgrade

Ausgabe von `k-playbook upgrade`, beim Aufruf dieses Commands schon gelaufen:

!`K_PLAYBOOK_VIA_COMMAND=1 k-playbook upgrade 2>&1`

Der Text dieses Commands steht unter `k-playbook:/commands/k-upgrade.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis
als `ref`, ersatzweise `k-playbook read k-playbook:/commands/k-upgrade.md` in der
Shell — und befolge ihn. Diese Datei selbst enthält sonst nichts.
