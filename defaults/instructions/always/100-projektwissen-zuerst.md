## Projektwissen zuerst

Die maßgebliche Projektdokumentation ist `keph:/docs/`, das Verzeichnis in der Collection
des Projekts; das Briefing nennt es unter `Projektdoku`. Lies sie, bevor du den Code
analysierst. Erst wenn die Dokumentation fehlt, nicht passt oder ein konkreter Fix den
aktuellen Code verlangt, ist eine Code-Recherche nötig.

So liest du den Einstieg: zuerst `keph:/docs/index.md`. Solange es sie nicht gibt, nimmst
du Kephalaion `list` auf `docs/` mit `recursive` und `frontmatter` und liest die Dokumente,
die zur Frage passen, einzeln mit `read`. Erzeugtes liegt unter `generated/` (etwa
`generated/versions/inventory.md`), Handgeschriebenes unter `manual/`.

Steht `kephalaion.ok` nicht auf `true`, ist die Projektdoku nicht erreichbar. Sag das,
statt auszuweichen; einen Rückfall auf `k-playbook-local/docs/` gibt es nicht.
