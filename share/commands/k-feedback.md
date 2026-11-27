---
description: Zeigt die eigenen Meldungen (Fehler und Wünsche, die jemand anderes erledigt) und zählt die offenen in jeder anderen lesbaren Collection; mit einem Text legt er eine Meldung unter keph:/feedback/ an, mit done, ablehnen oder annehmen und einer Nummer ändert er ihren Status. Dünne Hülle über dem Text k-playbook:/commands/k-feedback.md.
argument-hint: "[alle | neu | text | done <nr> | ablehnen <nr> | annehmen <nr> | offen <nr>]"
# model: github-copilot/gpt-5.5
allowed-tools: [Bash, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, mcp__k-playbook__k_playbook_feedback_list, mcp__k-playbook__k_playbook_feedback_create, mcp__k-playbook__k_playbook_feedback_update]
---

# k-feedback

Der Text dieses Commands steht unter `k-playbook:/commands/k-feedback.md`. Lies ihn
nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem Verweis als
`ref`, ersatzweise `k-playbook read k-playbook:/commands/k-feedback.md` in der
Shell — und befolge ihn. Das Argument des Aufrufs ist `$ARGUMENTS`; ist es leer, wurde
keins übergeben. Diese Datei selbst enthält sonst nichts.
