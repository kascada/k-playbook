---
description: Erzeugt semantische Projektdoku aus dem Code und schreibt sie über k-playbook docs checkout/publish in die Collection des Projekts nach keph:/docs/generated/code/. Scope ist project.repoRoot; ein Argument ergänzt genau einen Pfad innerhalb des Projekts. Dünne Hülle über dem Text k-playbook:/commands/k-docs-code.md.
argument-hint: [zusatz-pfad]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_docs_checkout, mcp__k-playbook__k_playbook_docs_publish, mcp__kephalaion__list, mcp__kephalaion__read]
---

# k-docs-code

Der Text dieses Commands steht unter `k-playbook:/commands/k-docs-code.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis als
`ref`, ersatzweise `k-playbook read k-playbook:/commands/k-docs-code.md` in der
Shell — und befolge ihn mit denselben `$ARGUMENTS`. Diese Datei selbst enthält sonst
nichts.
