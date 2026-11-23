---
title: Projektregeln nicht in CLAUDE.md und AGENTS.md
---

## Projektregeln nicht in CLAUDE.md und AGENTS.md

`CLAUDE.md` und `AGENTS.md` sind keine Ablage für Projektregeln, Notizen oder Hinweise.
`CLAUDE.md` ist nur der Include mit der Zeile `@AGENTS.md`, `AGENTS.md` trägt nur den Anstoß
zum Briefing. Schreibst du dort etwas hinein, auch über `/memory` oder `#`, erreicht es nicht
jeden Assistenten. Eine Projektregel gehört in die Instructions der Collection
(`keph:/k-playbook/instructions/`); wohin genau, steht in
`k-playbook:/instructions/on-demand/k-playbook.md`.
