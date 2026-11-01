---
description: Legt aus dem laufenden Gespräch einen neuen Task als Dokument in der Projekt-Collection an (keph:/user/~/tasks/), zeigt den Entwurf vorher und schreibt ihn erst nach Bestätigung; das Argument ist der Kurzname. Dünne Hülle über dem Text k-playbook:/commands/k-task-create.md.
argument-hint: [short-name]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_doc_create, mcp__k-playbook__k_playbook_doc_list, mcp__kephalaion__read, mcp__kephalaion__write]
---

# k-task-create

Der Text dieses Commands steht unter `k-playbook:/commands/k-task-create.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis als
`ref`, ersatzweise `k-playbook read k-playbook:/commands/k-task-create.md` in der
Shell — und befolge ihn. Das Argument des Aufrufs ist `$ARGUMENTS`. Diese Datei selbst
enthält sonst nichts.
