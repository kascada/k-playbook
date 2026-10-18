---
title: Vor dem Aufruf von gh
description: Die Regeln vor jedem Aufruf der GitHub CLI — Entscheidung und Host-Befund aus context.gh prüfen und das Konto nennen, bevor nach GitHub geschrieben wird. Nur für Commands, die gh rufen.
---

**Prüfe `context.gh` aus dem Briefing, bevor du `gh` aufrufst.** Halte an und melde es, statt
`gh` zu rufen, wenn `context.gh.status` auf `disabled` steht (das Projekt hat sich dagegen
entschieden), wenn er auf `unknown` steht (niemand hat entschieden — verweise auf `/k-gui`)
oder wenn `context.gh.ready` falsch ist. Im letzten Fall nenne, welche Hälfte fehlt: `gh`
installieren oder `gh auth login --hostname github.com` ausführen. Installiere nie selbst und
melde dich nie selbst an — beides ändert den Rechner, und die Anmeldung braucht einen Browser.

**Nenne `context.gh.account`, bevor du nach GitHub schreibst.** Freigeben, Mergen oder
Kommentieren ist nach außen sichtbar und geschieht als der, der angemeldet ist. Das aktive
Konto gilt rechnerweit und kann seit dem letzten Mal in einem anderen Projekt umgeschaltet
worden sein.
