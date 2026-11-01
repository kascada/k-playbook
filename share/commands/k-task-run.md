---
description: Führt Tasks aus der Projekt-Collection aus — ohne Argument die eigenen offenen Tasks in Nummernreihenfolge, mit Argument einen keph:-Verweis oder eine Tasknummer. Ausführung, Diff und Code-Review laufen in Sub-Agenten, die ihre Ergebnisse selbst an den Task anhängen; ein erfolgreich abgeschlossener Task wandert nach done/, sofern kein kritischer Review-Befund den Lauf vorher anhält. Dünne Hülle über dem Text k-playbook:/commands/k-task-run.md.
argument-hint: [ref-or-number]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite, Task, Agent, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_doc_list, mcp__k-playbook__k_playbook_doc_grep, mcp__k-playbook__k_playbook_doc_append, mcp__k-playbook__k_playbook_doc_replace, mcp__k-playbook__k_playbook_doc_set_field, mcp__k-playbook__k_playbook_doc_done, mcp__k-playbook__k_playbook_notify]
---

# k-task-run

Der Text dieses Commands steht unter `k-playbook:/commands/k-task-run.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis als
`ref`, ersatzweise `k-playbook read k-playbook:/commands/k-task-run.md` in der
Shell — und befolge ihn. Das Argument des Aufrufs ist `$ARGUMENTS`. Diese Datei selbst
enthält sonst nichts.
