---
description: Zeigt den Stand der Projektdoku unter keph:/docs/ — Inventar, Doku aus dem Code, Steckbriefe der Libraries und Tools, Handdoku —, meldet Lücken, alte Köpfe und Abweichungen der Steckbriefe vom Inventar und bietet die Erzeuger an; mit alle laufen Inventar, Code und Tools nacheinander. Ohne Wahl read-only. Dünne Hülle über dem Text k-playbook:/commands/k-docs.md.
argument-hint: [status|inventory|code|tools|alle] [zusatz-pfad]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, WebFetch, TodoWrite, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_docs_checkout, mcp__k-playbook__k_playbook_docs_publish, mcp__k-playbook__k_playbook_docs_exclude_add, mcp__kephalaion__list, mcp__kephalaion__read]
---

# k-docs

Der Text dieses Commands steht unter `k-playbook:/commands/k-docs.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis als
`ref`, ersatzweise `k-playbook read k-playbook:/commands/k-docs.md` in der
Shell — und befolge ihn mit denselben `$ARGUMENTS`. Diese Datei selbst enthält sonst
nichts.
