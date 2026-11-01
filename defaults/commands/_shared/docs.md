---
title: Die Docs-Ablage in der Projekt-Collection
description: Was für jeden Docs-Command gilt — die Ablage unter keph:/docs/ in der Collection des Projekts, ein Schreiber ändert nur seine Dokumente in generated/<erzeuger>/, Erzeuger mit mehreren Dokumenten schreiben über k-playbook docs checkout/publish, hints und questions im Frontmatter, README.md und index.md schreibt kein Command-Text, und ohne Kephalaion gibt es keinen Rückfall auf die Platte.
---

Die Projektdokumentation liegt in der Collection des Projekts unter `keph:/docs/`; das ist
der Teil der Collection, den Kephalaion indiziert. Darunter gilt:

- `docs/generated/<erzeuger>/` ist das Verzeichnis eines Erzeugers. Es kann mehrere
  Schreiber haben — in `code/` etwa `/k-docs-code` und `ks-overlay-repo-analyse`, erkennbar
  an `generated.by` im Frontmatter. Ein Lauf ändert nur die Dokumente, für die er zuständig
  ist; alle anderen bleiben unverändert. Ein Docs-Command schreibt nur in das Verzeichnis
  seines Erzeugers und nirgends sonst unter `docs/`.
- Ein Erzeuger mit mehreren Dokumenten schreibt über `k_playbook_docs_checkout` und
  `k_playbook_docs_publish` (`k-playbook docs checkout|publish <erzeuger>`): `checkout`
  holt das Verzeichnis in ein Arbeitsverzeichnis unter `.k-playbook/work/docs/<erzeuger>/`,
  dort wird mit den Datei-Werkzeugen geschrieben, `publish` gleicht ab und meldet, was
  inzwischen in Kephalaion geändert wurde, statt es zu überschreiben. Was im
  Arbeitsverzeichnis fehlt, löscht `publish` — ein Dokument, das der Lauf nicht behandelt,
  bleibt deshalb dort liegen.
- Im Frontmatter eines erzeugten Dokuments sind `hints` der einzige von Hand gepflegte
  Teil: kurze Sätze, worauf der nächste Lauf achten soll. Der Erzeuger befolgt sie, übernimmt
  sie unverändert und schlägt neue nur vor; `publish` lässt geänderte `hints` nur mit
  `allow_hints` durch, nach Bestätigung durch den Nutzer. `questions` nennt die Fragen, die
  genau dieses Dokument beantwortet.
- `docs/manual/` ist von Hand gepflegt; `docs/sources/` verlinkt Quellen außerhalb von
  Kephalaion. Beides fasst ein Erzeuger nicht an.
- `README.md` und `index.md` schreibt kein Command-Text: Die `README.md` von
  `generated/<erzeuger>/` schreibt `publish` aus der Erzeugertabelle im Programm, `index.md`
  entsteht später beim Indizieren. Auch kein Eintrag „schon einmal“ — er wäre beim nächsten
  Schreiben weg.
- Gelesen und geschrieben wird über die Verweisregel aus dem Briefing: `keph:`-Verweise mit
  `k_playbook_doc_read` bzw. Kephalaion `read`, Collection = `context.kephalaion.address`.
- Steht `context.kephalaion.ok` nicht auf `true`, ist die Ablage nicht erreichbar. Dann gibt es
  kein Ergebnis und keinen Rückfall auf die Platte: nichts nach `k-playbook-local/docs/`
  schreiben, nichts von dort als gültig lesen. Sag, dass Kephalaion nicht bereit ist, nenne
  `context.kephalaion.message`, und verweise auf die Projekt-Collection auf der Setup-Seite
  (`/k-gui`).
