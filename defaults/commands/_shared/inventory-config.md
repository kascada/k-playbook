---
title: Die Quellenkonfiguration des Versionsinventars schreiben
description: Wie ein Command die Quellenkonfiguration keph:/k-playbook/inventory.md in der Collection des Projekts anlegt oder ergänzt — das Gerüst eines neuen Dokuments (Frontmatter mit schema_version 3, sources, exclude, helm_values; Rumpf, der sie erklärt), die Schreibweise der Listen, der Weg read → Diff → create oder write mit base_revision samt stale_revision, name_taken und outcome_unknown, und die Prüfung danach über das Briefing in der Shell (k-playbook briefing --json).
---

Die Quellenkonfiguration ist das Dokument `keph:/k-playbook/inventory.md` in der Collection
des Projekts: die Einstellungen im Frontmatter (`schema_version`, `exclude`, `sources`,
`helm_values`), der Rumpf erklärt sie. Gelesen wird nur unterhalb der Projektwurzel; jeder
`path` steht relativ zu ihr, und `roots` gibt es nicht — ein Dokument mit `roots`, auch leer,
lehnt das Werkzeug ab.

**Gerüst.** Ein neues Dokument bekommt genau dieses Gerüst, die Einträge in den Listen:

```markdown
---
title: "Quellen des Versionsinventars"
schema_version: 3
sources: []
exclude: []
helm_values: []
---

# Quellen des Versionsinventars

Einstellungen für `k-playbook inventory`, im Frontmatter oben: zusätzliche Quellen über die
Standarderkennung hinaus (`sources`), Bereiche, in denen sie nicht sucht (`exclude`), und
Helm-Werte, die keine Image-Referenz sind (`helm_values`). Gelesen wird nur unterhalb der
Projektwurzel; jeder Pfad steht relativ zu ihr. Ohne Einträge gelten die Standardquellen.

Dieses Dokument ist handgepflegt. `/k-doc-inventory` ergänzt es nur nach ausdrücklicher
Bestätigung; bestehende Einträge, Kommentare und Reihenfolge bleiben erhalten. Den Vertrag
beschreibt `docs/version-inventory.md` der Installation, Abschnitt „Source Configuration“.
```

Eine Liste mit Einträgen steht als Block (`exclude:` und darunter `  - "<muster>"`), nicht als
`[]`; Muster mit `*` am Anfang stehen in doppelten Anführungszeichen. Ein Kommentar, der zu
einem Eintrag gehört, steht als eigene Zeile darüber, eingerückt wie der Eintrag. In einem
bestehenden Dokument kommt ein neuer Eintrag ans Ende seiner Liste; fehlt die Liste, kommt sie
ans Ende des Frontmatters, vor die schließende `---`-Zeile. Sonst ändert sich nichts — kein
Umsortieren, kein Neuformatieren, keine Kommentare entfernt, der Rumpf unangetastet.

**Schreiben.**

1. Lesen: Kephalaion `read` mit `collection` gleich `context.kephalaion.address` und `name`
   gleich `k-playbook/inventory.md`; merke dir die `revision`. Kommt `kind: none`, fehlt das
   Dokument.
2. Den Diff gegen den gelesenen Text zeigen — bei einem neuen Dokument den ganzen Inhalt — und
   bestätigen lassen. Ohne Bestätigung wird nicht geschrieben.
3. Fehlt das Dokument, Kephalaion `create` mit demselben `collection` und `name`; sonst
   Kephalaion `write` mit `base_revision` gleich der gemerkten Revision. Kommt
   `stale_revision` oder beim Anlegen `name_taken`, hat jemand anders inzwischen geschrieben:
   neu lesen, den Diff neu zeigen und neu bestätigen lassen, nie über den fremden Stand
   schreiben. Kommt `outcome_unknown`, erst mit `read` nachsehen, ob der Text angekommen ist,
   bevor noch einmal geschrieben wird.
4. Prüfen: `k-playbook briefing --json` in der Shell — nicht das Briefing vom Anfang, das den
   alten Stand trägt; `context.versionSources.state` steht auf `present` und trägt die neuen
   Einträge. Steht er auf `invalid`, das Dokument **nicht** selbst nachbessern: die Meldung aus
   `context.versionSources.error` nennen und den Nutzer entscheiden lassen.
