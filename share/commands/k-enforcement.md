---
description: Vorübergehend nicht verfügbar (Neuaufbau ohne Checks) — Prüft die laufende Änderung — die geänderten Dateien des Arbeitsbaums samt unverfolgten — auf den Doku-Nachzug und berichtet; reiht nach Freigabe einen Doku-Auftrag ein. Ein Argument grenzt den Prüfumfang auf ein Verzeichnis ein. Dünne Hülle über dem Text k-playbook:/commands/k-enforcement.md.
argument-hint: "[verzeichnis]"
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_doc_create, mcp__k-playbook__k_playbook_docs_order_add, mcp__k-playbook__k_playbook_doc_list, mcp__k-playbook__k_playbook_doc_grep, mcp__kephalaion__list, mcp__kephalaion__read, mcp__kephalaion__write]
---

# k-enforcement

## Vorübergehend nicht verfügbar

Dieser Command wird neu aufgebaut, seit k-playbook keine Checks mehr ausliefert, und steht
bis dahin nicht zur Verfügung. Führe nichts von dem aus, was darunter steht — nichts lesen,
nichts schreiben, nichts anbieten. Sag dem Nutzer in einem Satz, dass `/k-enforcement`
gerade neu aufgebaut wird und erst danach wieder geht. Den Doku-Nachzug leistet bis dahin
jede Sitzung selbst, nach der Anweisung „Doku nachziehen“ im Briefing.

Der Text darunter bleibt als Vorlage für den Neuaufbau stehen.

Der Text dieses Commands steht unter `k-playbook:/commands/k-enforcement.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis als
`ref`, ersatzweise `k-playbook read k-playbook:/commands/k-enforcement.md` in der
Shell — und befolge ihn mit denselben `$ARGUMENTS`. Diese Datei selbst enthält sonst
nichts.
