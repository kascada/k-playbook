---
description: Vorübergehend nicht verfügbar (Umzug der Ergebnisablage, voraussichtlich bis 2026-11-13) — Arbeitet Befunde aus einer Review-Ergebnisdatei strukturiert ab. Plant zuerst sinnvolle Remediation-Bündel nach Risiko, Aufwand und Kopplung, beachtet die projektlokale Remediation-Policy und erzeugt je nach Modus Tasks in der Projekt-Collection statt direkte Fixes. Dünne Hülle über dem Text k-playbook:/commands/k-remediation.md.
argument-hint: [result-datei.md]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_doc_create, mcp__k-playbook__k_playbook_doc_list, mcp__k-playbook__k_playbook_doc_grep, mcp__kephalaion__read, mcp__kephalaion__write]
---

# k-remediation

## Vorübergehend nicht verfügbar

Dieser Command wird auf die neue Ablage für Ergebnisse umgebaut und steht bis dahin nicht
zur Verfügung, voraussichtlich bis zum 13. November 2026. Führe nichts von dem aus, was
darunter steht — nichts lesen, nichts schreiben, nichts anbieten. Sag dem Nutzer in einem
Satz, dass `/k-remediation` gerade umzieht und voraussichtlich ab dem 13. November 2026
wieder geht. Vorhandene `review-triage.md` unter `k-playbook-local/results/` bleiben
unverändert liegen und lassen sich danach wieder abarbeiten.

Der Text darunter bleibt als Vorlage für den Umbau stehen.

Der Text dieses Commands steht unter `k-playbook:/commands/k-remediation.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis als
`ref`, ersatzweise `k-playbook read k-playbook:/commands/k-remediation.md` in der
Shell — und befolge ihn. Das Argument des Aufrufs ist `$ARGUMENTS`. Diese Datei selbst
enthält sonst nichts.
