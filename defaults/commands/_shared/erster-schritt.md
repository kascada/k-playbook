---
title: Erster Schritt jedes Commands
description: Wird von jedem Command als erstes Include eingebunden — das Briefing der Sitzung nehmen oder holen; was darüber hinaus für jeden Command zu Beginn gilt, kommt hierher.
---

Dieser Baustein ist der erste Schritt jedes Commands; jeder Command-Text beginnt mit
`include: _shared/erster-schritt.md`.

Liegt das Briefing in dieser Sitzung vor, nimm es. Sonst rufe `k_playbook_briefing` mit
`projectDir` = Arbeitsverzeichnis, ersatzweise `k-playbook briefing --json`. Ist k-playbook
nicht installiert, brich ab und melde es. Die Regeln zum Briefing stehen in seinem Text der
immer geltenden Anweisungen (`documents.always.content`).
