---
title: Die Docs-Ablage in der Projekt-Collection
description: Was für jeden Docs-Command gilt — die Ablage unter keph:/docs/ in der Collection des Projekts, ein Erzeuger schreibt nur sein generated/<erzeuger>/, README.md und index.md schreibt nie ein Command, und ohne Kephalaion gibt es keinen Rückfall auf die Platte.
---

Die Projektdokumentation liegt in der Collection des Projekts unter `keph:/docs/`; das ist
der Teil der Collection, den Kephalaion indiziert. Darunter gilt:

- `docs/generated/<erzeuger>/` gehört genau einem Erzeuger, und der ersetzt sein Verzeichnis
  bei jedem Lauf als Ganzes. Ein Docs-Command schreibt nur in sein eigenes
  `generated/<erzeuger>/` und nirgends sonst unter `docs/`.
- `docs/manual/` ist von Hand gepflegt; `docs/sources/` verlinkt Quellen außerhalb von
  Kephalaion. Beides fasst ein Erzeuger nicht an.
- `README.md` und `index.md` je Verzeichnis erzeugt Kephalaion. Kein Command schreibt sie —
  auch nicht, um „schon einmal“ einen Eintrag zu ergänzen; er wäre beim nächsten Erzeugen weg.
- Gelesen und geschrieben wird über die Verweisregel aus dem Briefing: `keph:`-Verweise mit
  `k_playbook_doc_read` bzw. Kephalaion `read`, Collection = `context.kephalaion.address`.
- Steht `context.kephalaion.ok` nicht auf `true`, ist die Ablage nicht erreichbar. Dann gibt es
  kein Ergebnis und keinen Rückfall auf die Platte: nichts nach `k-playbook-local/docs/`
  schreiben, nichts von dort als gültig lesen. Sag, dass Kephalaion nicht bereit ist, nenne
  `context.kephalaion.message`, und verweise auf die Projekt-Collection auf der Setup-Seite
  (`/k-gui`).
