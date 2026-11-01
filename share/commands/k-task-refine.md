---
description: Härtet Tasks aus der Projekt-Collection und Instruction-Dateien vor der Ausführung mit einem read-only Critic/Editor-Dialog; ohne Argument die eigenen offenen Tasks, mit Argument ein keph:-Verweis, eine Tasknummer oder eine Datei auf der Platte. Dünne Hülle über dem Text k-playbook:/commands/k-task-refine.md.
argument-hint: [ref-or-path]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, Task, Agent, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_doc_list, mcp__k-playbook__k_playbook_doc_grep, mcp__k-playbook__k_playbook_doc_replace, mcp__k-playbook__k_playbook_doc_append, mcp__k-playbook__k_playbook_notify]
---

# k-task-refine

Der Text dieses Commands steht unter `k-playbook:/commands/k-task-refine.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis als
`ref`, ersatzweise `k-playbook read k-playbook:/commands/k-task-refine.md` in der
Shell — und befolge ihn. Das Argument des Aufrufs ist `$ARGUMENTS`. Diese Datei selbst
enthält sonst nichts.
