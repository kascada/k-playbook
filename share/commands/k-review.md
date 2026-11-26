---
description: Führt ein einzelnes Review-Rezept aus — ohne Argument Auswahl aus den für /k-review aktivierten Rezepten. Ein Bericht-Rezept erhebt seine Funde als SARIF-Fundliste in ein Arbeitsverzeichnis (zweites Argument, sonst ein neues unter /tmp/ nur für den eigenen Benutzer), fasst sie über den Merge zusammen und schreibt die Ergebnisdatei review-triage.md als Abhakliste nach Priorität; ein interaktives Rezept ändert den Code Stelle für Stelle nach Freigabe. Dünne Hülle über dem Text k-playbook:/commands/k-review.md.
argument-hint: "[review-name] [arbeitsverzeichnis]"
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_review_workdir, mcp__k-playbook__k_playbook_review_merge_dir]
---

# k-review

Der Text dieses Commands steht unter `k-playbook:/commands/k-review.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis als
`ref`, ersatzweise `k-playbook read k-playbook:/commands/k-review.md` in der
Shell — und befolge ihn mit denselben `$ARGUMENTS`. Diese Datei selbst enthält sonst
nichts.
