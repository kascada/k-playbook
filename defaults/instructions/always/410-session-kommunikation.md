---
title: "Session-übergreifende Zusammenarbeit"
description: "OpenCode kann unabhängige Sessions über die vorhandene API ansprechen; der Zugriff erfolgt nur im Rahmen des Nutzerauftrags."
---

## Session-übergreifende Zusammenarbeit

### OpenCode

OpenCode V2 kann unabhängig gestartete Sessions desselben Servers über seine vorhandene
Session-API lesen und ansprechen und deren Verarbeitung auslösen. Dafür ist kein eigener
Vermittlungsdienst erforderlich. Das Fehlen eines direkt angebotenen Werkzeugs ist kein Beleg,
dass die Fähigkeit fehlt: Prüfe vor einer solchen Aussage die dokumentierte API und den
Zugriff der angebundenen Installation. Nutze angebotene OpenCode-Werkzeuge, soweit sie passen,
sonst `opencode api` mit dessen Server-Erkennung und Authentifizierung.

Lies vor einer Session-Übergabe, Rundfrage oder Arbeitsdelegation
`k-playbook:/instructions/on-demand/session-kommunikation.md`. Der Command
`/k-session-contact` bereitet die eigene Session als Ansprechpartner vor und gibt eine
Kontaktkarte aus. Er ist eine Kontaktvereinbarung, keine technische Zugriffssperre und keine
Voraussetzung der API. Fremde Sessions werden nur im Rahmen des Nutzerauftrags gelesen oder
angesprochen. Werkzeuge, Tests und Änderungen sind nach konkretem Auftrag und bestehenden
Berechtigungen zulässig; es gilt kein pauschaler Nur-Antworten-Modus.

### Claude

Das Verfahren für unabhängige Claude-Sessions ist noch nicht definiert. Die OpenCode-API und
ihre Session-IDs werden nicht auf Claude übertragen. Verfügbare Subagenten-Werkzeuge sind ein
anderer Mechanismus und kein Nachweis unabhängiger Session-Kommunikation.
