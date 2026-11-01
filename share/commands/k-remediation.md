---
description: Arbeitet Befunde aus einer Review-Ergebnisdatei strukturiert ab. Plant zuerst sinnvolle Remediation-Bündel nach Risiko, Aufwand und Kopplung, beachtet die projektlokale Remediation-Policy und erzeugt je nach Modus Tasks in der Projekt-Collection statt direkte Fixes. Dünne Hülle über dem Text k-playbook:/commands/k-remediation.md.
argument-hint: [result-datei.md]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_doc_create, mcp__k-playbook__k_playbook_doc_list, mcp__k-playbook__k_playbook_doc_grep, mcp__kephalaion__read, mcp__kephalaion__write]
---

# k-remediation

Der Text dieses Commands steht unter `k-playbook:/commands/k-remediation.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis als
`ref`, ersatzweise `k-playbook read k-playbook:/commands/k-remediation.md` in der
Shell — und befolge ihn. Das Argument des Aufrufs ist `$ARGUMENTS`. Diese Datei selbst
enthält sonst nichts.
