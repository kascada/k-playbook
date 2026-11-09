---
description: Zieht Installation und Projekt nach — aktualisiert das Programm, wenn es älter ist als die Installation, bringt AGENTS.md, CLAUDE.md, Verlinkung und MCP-Einträge auf den Stand und führt danach die Umstellungen durch, die eine Beurteilung brauchen — zuerst die Reste der Wissensablage auf der Platte (3.0: cache, inbox und queue werden ohne Rückfrage gelöscht, jede Datei aus knowledge wird zum Umzug nach keph:/docs/manual/ oder zum Löschen vorgelegt), dann die alten Befunde aus material/befunde/, die Doku aus k-playbook-local/docs/, die je Bereich nach keph:/docs/ zieht oder abgeräumt wird, und die Tasks aus k-playbook-local/tasks/, die unter gleichem Namen nach keph:/user/<user>/tasks/ umziehen — und räumt zuletzt die Verzeichnisse unter k-playbook-local/ ab, die das Einrichten nicht mehr anlegt (docs, tasks, data, material, commands, skills, rules, checks), wenn darin nur noch README.md und die verwaltete .gitignore liegen, in data zudem die umbenannten Reste der Todo-Übernahme (todos.json.migriert, todos.json.nachzuegler-*); ein noch nicht übernommenes todos.json hält data fest, und liegt in commands oder skills noch mehr, nennt er jeden Eintrag mit seinem Ziel unter .k-playbook/ und zieht nur auf ausdrücklichen Zuruf um.
# model: github-copilot/gpt-5.5
allowed-tools: [Bash, Read, Edit, Grep, Glob, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_doc_create, mcp__k-playbook__k_playbook_doc_list, mcp__k-playbook__k_playbook_doc_grep, mcp__k-playbook__k_playbook_doc_append, mcp__k-playbook__k_playbook_doc_replace, mcp__kephalaion__read, mcp__kephalaion__list, mcp__kephalaion__create, mcp__kephalaion__write]
---

# k-upgrade

Ausgabe von `k-playbook upgrade`, beim Aufruf dieses Commands schon gelaufen:

!`k-playbook upgrade 2>&1`

Der Text dieses Commands steht unter `k-playbook:/commands/k-upgrade.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis
als `ref`, ersatzweise `k-playbook read k-playbook:/commands/k-upgrade.md` in der
Shell — und befolge ihn. Diese Datei selbst enthält sonst nichts.
