---
title: "Session-übergreifende Zusammenarbeit"
description: "OpenCode kann unabhängige Sessions über die vorhandene API ansprechen; der Zugriff erfolgt nur im Rahmen des Nutzerauftrags."
---

## Session-übergreifende Zusammenarbeit

### OpenCode

OpenCode V2 kann unabhängig gestartete Sessions desselben Servers über seine Session-API
ansprechen: mit den angebotenen OpenCode-Werkzeugen, sonst mit `opencode api`. Fehlt ein direkt
angebotenes Werkzeug, ist das kein Beleg, dass die Fähigkeit fehlt. Fremde Sessions werden nur im
Rahmen des Nutzerauftrags gelesen oder angesprochen. Lies vor einer Session-Übergabe, Rundfrage
oder Arbeitsdelegation `k-playbook:/instructions/on-demand/session-kommunikation.md`;
`/k-session-contact` gibt die Kontaktkarte der eigenen Session aus.

### Claude

Für unabhängige Claude-Sessions ist noch kein Verfahren definiert. Die OpenCode-API und ihre
Session-IDs gelten dort nicht; Subagenten-Werkzeuge sind ein anderer Mechanismus.
