---
description: Arbeitet die offenen Doku-Aufträge für den Erzeuger code ab — legt abhängige zusammen, zerlegt sie in Aufträge je Dokument und schreibt diese Dokumente als semantische Projektdoku aus dem Code über k-playbook docs checkout/publish in die Collection des Projekts nach keph:/docs/generated/code/. Die Pfade der Aufträge bestimmen, welche Dokumente drankommen; ohne offene Aufträge läuft nichts, den vollen Lauf reiht /k-docs code ein. Nimmt kein Argument. Dünne Hülle über dem Text k-playbook:/commands/k-docs-code.md.
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_doc_list, mcp__k-playbook__k_playbook_doc_grep, mcp__k-playbook__k_playbook_doc_done, mcp__k-playbook__k_playbook_doc_create, mcp__k-playbook__k_playbook_docs_checkout, mcp__k-playbook__k_playbook_docs_publish, mcp__k-playbook__k_playbook_docs_exclude_add, mcp__kephalaion__list, mcp__kephalaion__read, mcp__kephalaion__write]
---

# k-docs-code

Der Text dieses Commands steht unter `k-playbook:/commands/k-docs-code.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis als
`ref`, ersatzweise `k-playbook read k-playbook:/commands/k-docs-code.md` in der
Shell — und befolge ihn mit denselben `$ARGUMENTS`. Diese Datei selbst enthält sonst
nichts.
