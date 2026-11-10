## Projektwissen zuerst

Die maßgebliche Projektdokumentation ist `keph:/docs/`, das Verzeichnis in der Collection
des Projekts; das Briefing nennt es unter `Projektdoku`. Lies sie, bevor du den Code
analysierst. Erst wenn die Dokumentation fehlt, nicht passt oder ein konkreter Fix den
aktuellen Code verlangt, ist eine Code-Recherche nötig.

So findest du, was zur Frage passt:

1. **Überblick:** `keph:/docs/index.md` mit Kephalaion `read` — je Verzeichnis eine
   `index.md`, die seinen Inhalt mit Titel und Beschreibung nennt.
2. **Volltextsuche:** Kephalaion `search` mit `query` in einfachen **englischen** Wörtern
   (die Doku ist englisch; eine Frage geht auch, Suchsyntax nicht), `collection` gleich
   `kephalaion.address`, wahlweise `path` wie `docs/manual`. Jeder Treffer nennt Dokument,
   Titel und Abschnitt mit `anchor`. Ist `search` nicht in deiner Werkzeugliste, lade es über
   die Werkzeugsuche, bevor du ausweichst.
3. **Lesen:** `read` mit `name` des Treffers und `section` gleich seinem `anchor` liefert nur
   diesen Abschnitt; ohne `section` das ganze Dokument.

Erzeugtes liegt unter `generated/` (etwa `generated/versions/inventory.md`), Handgeschriebenes
unter `manual/`. Findet die Suche nichts Passendes, geh über die `index.md` der Verzeichnisse.

Steht `kephalaion.ok` nicht auf `true`, ist die Projektdoku nicht erreichbar. Sag das,
statt auszuweichen; einen Rückfall auf `k-playbook-local/docs/` gibt es nicht.
