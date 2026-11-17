---
description: Ordnet die Befunde in keph:/queue/findings/ einzeln ein — prüft jeden gegen den geholten Hauptbranch, Doku, Tasks, Todos und Workbench, bringt ihn an seinen Ort (Handdoku, Todo, Nachtrag, Doku-Auftrag) oder löscht ihn, und fängt nur an, wenn die Vorprüfung im Programm alle Befunde aller Nutzer als lesbar meldet. Jeder Lauf hinterlässt ein Protokoll in keph:/queue/einordnung/. Das Argument ist die Obergrenze je Lauf (Standard 25). Dünne Hülle über dem Text k-playbook:/commands/k-befunde-einordnen.md.
argument-hint: [anzahl]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_findings_check, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_doc_list, mcp__k-playbook__k_playbook_doc_grep, mcp__k-playbook__k_playbook_doc_create, mcp__k-playbook__k_playbook_doc_append, mcp__k-playbook__k_playbook_doc_replace, mcp__k-playbook__k_playbook_todo_list, mcp__k-playbook__k_playbook_todo_add, mcp__k-playbook__k_playbook_todo_update, mcp__kephalaion__list, mcp__kephalaion__read, mcp__kephalaion__search, mcp__kephalaion__create, mcp__kephalaion__write, mcp__kephalaion__rename, mcp__kephalaion__delete]
---

# k-befunde-einordnen

Der Text dieses Commands steht unter `k-playbook:/commands/k-befunde-einordnen.md`. Lies
ihn nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis als
`ref`, ersatzweise `k-playbook read k-playbook:/commands/k-befunde-einordnen.md` in der
Shell — und befolge ihn mit denselben `$ARGUMENTS`. Diese Datei selbst enthält sonst
nichts.
