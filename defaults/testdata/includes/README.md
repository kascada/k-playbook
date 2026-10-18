# testdata/includes

Feste Testfälle für den Include-Mechanismus der Verweisauflösung
(`docs/verweisschema.md`, Abschnitt „Includes“). Dieselben Dateien dienen zwei
Zwecken:

- Die Go-Tests in `installer/internal/project/docref_test.go` lesen sie von der
  Platte; die Erwartungen stehen im Test, hier liegen nur die Eingaben.
- Nach einem Release liegen sie unter `k-playbook:/testdata/includes/<fall>.md` auf
  jedem Hub und in jeder Installation — nur lesbar, nirgends gelistet — und liefern
  dort dasselbe wie im Test: `k-playbook read k-playbook:/testdata/includes/einfach.md`.

Kein Fall bindet `project:` oder `keph:` ein: Die Fälle müssen auf jeder Installation
gleich ausgehen, und ein Projektziel gäbe es nach dem Release nirgends.

| Fall | Zeigt |
|---|---|
| `einfach.md` | eine Include-Zeile auf `baustein.md`; der Rumpf wird eingesetzt |
| `baustein.md` | der Baustein, mit Frontmatter, das im eingesetzten Text fehlt |
| `verschachtelt.md` | bindet `einfach.md` ein, das `baustein.md` einbindet — Tiefe 2 |
| `relativ.md`, `teile/stueck.md` | ein relatives Ziel im Unterverzeichnis |
| `zyklus-a.md`, `zyklus-b.md` | binden einander ein — `include_cycle` |
| `zu-tief-00.md` … `zu-tief-11.md` | eine Kette über die Grenze; ab `00` ist `11` Tiefe 11 — `include_too_deep`; ab `01` reicht sie genau bis Tiefe 10 |
| `tot.md` | ein Ziel, das es nicht gibt — `not_found` mit Kette |
| `codeblock.md` | Include-Zeilen in Code-Blöcken (``` und ~~~) bleiben stehen; nur die letzte wird eingesetzt |

Dateinamen ASCII, Inhalt UTF-8. Das Verzeichnis ist vom Versionsinventar ausgenommen
(`**/testdata/**`).
