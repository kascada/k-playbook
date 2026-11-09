---
description: Prüft die laufende Änderung — die geänderten Dateien des Arbeitsbaums samt unverfolgten — auf den Doku-Nachzug, führt k-check --mode changed über dieselbe Menge aus und berichtet beides; reiht nach Freigabe einen Doku-Auftrag ein. Ein Argument grenzt den Prüfumfang auf ein Verzeichnis ein. Dünne Hülle über dem Text k-playbook:/commands/k-enforcement.md.
argument-hint: [verzeichnis]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_doc_create, mcp__k-playbook__k_playbook_doc_list, mcp__k-playbook__k_playbook_doc_grep, mcp__kephalaion__list, mcp__kephalaion__read, mcp__kephalaion__write]
---

# k-enforcement

Der Text dieses Commands steht unter `k-playbook:/commands/k-enforcement.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis als
`ref`, ersatzweise `k-playbook read k-playbook:/commands/k-enforcement.md` in der
Shell — und befolge ihn mit denselben `$ARGUMENTS`. Diese Datei selbst enthält sonst
nichts.
