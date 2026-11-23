---
title: Die Docs-Ablage in der Projekt-Collection
description: Was für jeden Docs-Command gilt — die Ablage unter keph:/docs/ in der Collection des Projekts, ein Schreiber ändert nur seine Dokumente in generated/<erzeuger>/, Erzeuger mit mehreren Dokumenten schreiben über k-playbook docs checkout/publish, der eine Kopf jedes Dokuments (title, description, generated samt stats, hints, questions, sources, verified), die Verweise auf die Regeln eines schreibenden Laufs, auf den Scope und auf die Doku-Aufträge, dauerhafte Ausschlüsse in docs.exclude, README.md (aus den Vorlagen unter k-playbook:/docs/, geschrieben vom Programm) und index.md schreibt kein Command-Text, und ohne Kephalaion gibt es keinen Rückfall auf die Platte.
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
- Jedes Dokument trägt den Kopf aus dem Abschnitt „Der Kopf“ unten; `hints` sind darin der
  einzige von Hand gepflegte Teil eines erzeugten Dokuments.
- Schreibt ein Lauf selbst erzeugte Dokumente (nicht das Programm, wie beim Inventar),
  gelten zusätzlich die Regeln aus `k-playbook:/commands/_docs/schreiblauf.md`: Zeitpunkt,
  Unklarheiten, keine Befunde, was nicht in die Doku gehört, Sub-Agenten.
- Dauerhafte Ausschlüsse für den Code-Scan eines Projekts stehen in `docs.exclude` von
  `K-PLAYBOOK.yaml`: gelesen aus `context.docs.exclude`, eingetragen nur über
  `k_playbook_docs_exclude_add` (`k-playbook docs exclude add`) nach Bestätigung durch den
  Nutzer. Die Datei selbst liest und bearbeitet kein Docs-Command. Wie sie wirken, rechnet
  `k-playbook docs scope`; die Regeln stehen nur im Programm.
- `docs/manual/` ist von Hand gepflegt, samt `docs/manual/handbook/`, dem Handbuch des
  Projekts, das die Oberfläche zeigt; `docs/sources/` verlinkt Quellen außerhalb von
  Kephalaion. Beides fasst ein Erzeuger nicht an.
- `README.md` und `index.md` schreibt kein Command-Text: Die `README.md` eines Verzeichnisses
  unter `docs/` kommt aus ihrer Vorlage `k-playbook:/docs/<pfad>/README.md`, nicht aus einer
  Tabelle im Programm, und ist in jedem Projekt gleich. Geschrieben wird sie vom Programm —
  von `publish` und vom Inventar für ihr Verzeichnis, beim Einrichten und von
  `k-playbook upgrade` für alle, und von `k-playbook docs readme`
  (`k_playbook_docs_readme`) —, wenn sie fehlt oder abweicht; eine von Hand geänderte wird
  dabei überschrieben. Verzeichnisse ohne Vorlage und `docs/README.md` ganz oben fasst das
  Programm nicht an, dort mahnt Kephalaion. `index.md` entsteht beim Indizieren. Auch kein
  Eintrag „schon einmal“ — er wäre beim nächsten Schreiben weg.
- Gelesen und geschrieben wird über die Verweisregel aus dem Briefing: `keph:`-Verweise mit
  `k_playbook_doc_read` bzw. Kephalaion `read`, Collection = `context.kephalaion.address`.
- Steht `context.kephalaion.ok` nicht auf `true`, ist die Ablage nicht erreichbar. Dann gibt es
  kein Ergebnis und keinen Rückfall auf die Platte: nichts ersatzweise in eine Datei
  schreiben, keine Datei als gültig lesen. Sag, dass Kephalaion nicht bereit ist, nenne
  `context.kephalaion.message`, und verweise auf die Projekt-Collection auf der Setup-Seite
  (`/k-gui`).

### Der Kopf

Jedes Dokument unter `keph:/docs/` hat denselben Kopf, erzeugt oder von Hand geschrieben:
Kephalaion indiziert alles zusammen. Die Felder in dieser Reihenfolge:

```yaml
---
title: "<Titel>"
description: "<Ein Satz: was dieses Dokument erklärt.>"
generated: { by: <erzeuger>, at: <GENERATED_AT>, stats: { … } }
hints: [ "<worauf der nächste Lauf achten soll>", … ]
questions: [ "<Frage, die genau dieses Dokument beantwortet>", … ]
sources:
  - resource: <keph:-Verweis oder URL>
    item: <Eintrag darin>
    version: <Stand, gegen den gelesen wurde>
verified: <nur ein Mensch>
---
```

- `title` — Pflicht.
- `description` — Pflicht, ein Satz. Ohne `title` oder `description` lehnt `publish` ab und
  schreibt nichts.
- `generated` — nur bei erzeugten Dokumenten. `by` ist der Schreiber (`k-docs-code`,
  `ks-overlay-repo-analyse`, `k-docs-tools`, `k-doc-inventory`), `at` der Zeitpunkt des
  Laufs. `stats` ist optional und nimmt eigene Zahlen eines Erzeugers auf, etwa die Zählungen
  des Inventars; es ist kein Suchinhalt, das Indizieren übergeht es.
- `hints` — optional, der einzige von Hand gepflegte Teil eines erzeugten Dokuments: kurze
  Sätze, worauf der nächste Lauf achten soll. Der Erzeuger befolgt sie und übernimmt sie
  unverändert. Neue schlägt er nur vor und trägt sie erst nach Bestätigung ein; dasselbe gilt,
  wenn ein entfallendes Dokument `hints` hat. `publish` lässt geänderte `hints` nur mit
  `allow_hints` durch, nach Bestätigung durch den Nutzer.
- `questions` — die Fragen, die ein Neuer in der ersten Woche stellt und die genau dieses
  Dokument beantwortet. Eine Frage, eine Datei: Verteilt sich eine Antwort über mehrere
  Dokumente, ist der Zuschnitt falsch.
- `sources` — die Quellen außer dem Code, gegen die gelesen wurde; fehlt, wenn es keine gab.
  Je Eintrag ist `resource` Pflicht, ein `keph:`-Verweis oder eine URL; `item` nennt einen
  Eintrag darin, `version` den Stand, gegen den gelesen wurde.
- `verified` — trägt nur ein Mensch ein. Ein Erzeuger setzt es nie und übernimmt es nicht
  von sich aus.

Darüber hinaus schreibt kein Erzeuger ein Kopffeld. Die Felder älterer Fassungen — `type`,
`tags`, `status`, `lib`, `version`, `version-pin`, `severity`, `last-reviewed`, ein eigener
Block wie `inventory:` — entfallen oder gehen in diese Felder auf. Die Vorlage eines
Erzeugers zeigt, wie er diesen Kopf belegt; sie erweitert ihn nicht.
