# Docs-Modul: Inventar

Dieses Modul wird von `/k-docs` und `/k-doc-inventory` nach dem Shared Context angewendet.
Es ist kein eigenständiger Einstieg, sondern der nachladbare Ablauf für die Herkunft
`docs/versions/`.

Erhebt das Versionsinventar des Projekts: eine vollständige, reproduzierbare Übersicht der
**deklarierten** Versionen — Pakete, Tools, Runtimes, Container-Images, Helm-Charts und
Helm-Abhängigkeiten —, nach Umgebung getrennt und mit Herkunft je Zeile. Die Erhebung
selbst ist Go-Fachlogik: dieser Ablauf klärt die Quellenlage, stößt das Subkommando
`k-playbook inventory` an und berichtet dessen Ergebnis. Der Index über die erzeugte Datei
wird von `/k-docs-index` gebaut; dieses Modul schreibt ihn nicht.

Der verbindliche Vertrag steht in `<playbook.dir>/docs/versionsinventar.md`: Datenmodell,
Pin-Taxonomie, Quellenarten, Abweichungen, Vertrauensgrenze, Quellenkonfiguration und die
Regel zu Zeitstempel und Byte-Stabilität. Dieses Modul formuliert nichts davon neu.

Produces:
- `k-playbook-local/docs/versions/inventory.md` — geschrieben vom Subkommando, nicht von
  Hand. Ein Lauf ohne inhaltliche Änderung lässt die Datei byte-identisch stehen.
- `k-playbook-local/version-sources.yaml` — **nur** nach ausdrücklicher Bestätigung des
  Nutzers und ausschließlich ergänzend (Schritt 4).

Nichts sonst.

## Schritt 1 — Pfade auflösen

From the context output:

- `RESOLVED_DOCS_DIR = <local.dir>/docs`
- `DOCS_DISPLAY_PATH = k-playbook-local/docs`
- `VERSIONS_DIR = <RESOLVED_DOCS_DIR>/versions`
- `VERSIONS_DISPLAY_PATH = k-playbook-local/docs/versions`
- `INVENTORY_FILE = <VERSIONS_DIR>/inventory.md`
- `INVENTORY_DISPLAY_PATH = k-playbook-local/docs/versions/inventory.md`
- `VERSION_SOURCES_PATH = versionSources.path` aus der Context-Ausgabe
- `VERSION_SOURCES_DISPLAY_PATH = k-playbook-local/version-sources.yaml`

Command-specific policy:

- If `RESOLVED_DOCS_DIR` is missing on disk: ask whether to create exactly that directory
  now or to run `/k-gui`. Do not use a fallback path and do not abort hard.
- `VERSIONS_DIR` is this origin's producer directory. Es fehlt vor dem ersten Lauf, und das
  ist der Normalzustand: das Subkommando legt es selbst an. Nicht vorher anlegen, nicht
  danach fragen.
- `VERSION_SOURCES_PATH` gehört zur Struktur aus dem Einrichten. Fehlt die Datei, ist das
  **kein** Fehler — dann gelten die Standardquellen unterhalb der Projektwurzel. Wird sie
  gebraucht (Schritt 4), fragen, ob `/k-gui` laufen soll: es legt die gültige, leere
  Konfiguration aus der Vorlage des Werkzeugs an. Die Vorlage nicht selbst nachbauen.
- Geschrieben wird ausschließlich nach `VERSIONS_DIR` — und dort nur vom Subkommando —
  sowie nach Schritt 4 in `VERSION_SOURCES_PATH`. `docs/README.md` gehört allein
  `/k-docs-index`; `docs/code/`, `docs/libs/`, `docs/extracted/` und `docs/manual/` gehören
  anderen Erzeugern und werden hier nicht angefasst.

## Schritt 2 — Quellenlage sichten

Zwei Auskünfte, beide ohne eigene Dateilektüre:

1. **Die Quellenkonfiguration** aus dem Feld `versionSources` der Context-Ausgabe. Lies
   `VERSION_SOURCES_PATH` **nicht** selbst — dieselbe Regel wie für `K-PLAYBOOK.yaml`. Drei
   Zustände, und mehr nicht:
   - `present: true`, `error` leer → gültig. `roots`, `sources` und `exclude` tragen den
     Inhalt; leere Listen heißen „nichts konfiguriert".
   - `present: false` → fehlt. Es gelten die Standardquellen unterhalb der Projektwurzel.
   - `error` gefüllt → defekt. Der Erhebungslauf bricht damit ab (Fehlerfälle unten).
   Fehlt das Feld `versionSources` ganz, ist die Installation älter als es: das sagen und
   auf ein Update verweisen, statt die Datei ersatzweise selbst zu öffnen.
2. **Der Bestand des Inventars**: existiert `INVENTORY_FILE`, lies **nur** den
   YAML-Block zwischen den beiden `---`-Zeilen am Dateianfang — `generated.at` und die
   Zahlen unter `inventory:`. Der Markdown-Rumpf wird nie geparst.

Zeige die Lage kompakt:

```text
/k-doc-inventory — Quellenlage
─────────────────────────────────────
Inventar      erhoben <generated.at>, <N> Quellen, <N> Abweichungen | fehlt
Quellkonfig   vorhanden (<N> Wurzeln, <N> Quellen, <N> Ausschlüsse) | fehlt | defekt: <error>
```

## Schritt 3 — Weitere Quellen erfragen

Nur beim ersten Lauf — also wenn `INVENTORY_FILE` fehlt — oder wenn die
Quellenkonfiguration fehlt beziehungsweise keine `sources` führt. Sonst diesen Schritt
überspringen: ein Command, der bei jedem Lauf dieselbe Frage stellt, wird weggeklickt.

**Gebündelt, in einer Nachricht** — kein Ping-Pong:

```text
Das Inventar erhebt die Standardquellen unterhalb der Projektwurzel: Manifeste,
Lockfiles, Dockerfiles, Compose, DevContainer, Helm und CI.

1. Gibt es weitere Versionsquellen? Host-Dateien, eigene Deployment-Repos,
   `.tool-versions`, Nix-, Terraform-, Ansible- oder proprietäre Konfigurationen.
2. Welche davon stehen für `lokal`, `dev`, `devcontainer`, `ci` oder Deployment?
3. Sollen sie künftig automatisch mitgescannt werden — also in
   k-playbook-local/version-sources.yaml eingetragen werden?
4. Gibt es umgekehrt Bereiche, die *nicht* mitgescannt werden sollen? Testfixtures
   und Beispielprojekte etwa, deren Versionen absichtlich alt oder widersprüchlich
   sind und über das Projekt nichts aussagen.

Ohne Angabe läuft die Erhebung mit den Standardquellen; das ist eine gültige
Antwort und kein Mangel.
```

Die Installation `k-playbook/` ist immer ausgenommen und gehört nicht in Frage 4 — das
regelt der Vertrag, nicht die Konfiguration.

Antwortet der Nutzer mit Quellen, ordne jeder eine Quellart (`kind`) und ein Umgebungslabel
(`env`) aus den geschlossenen Mengen des Vertrags zu und zeige die Zuordnung, bevor
irgendetwas geschrieben wird. Liegt eine Quelle außerhalb der Projektwurzel, braucht sie
zusätzlich eine Wurzel unter `roots:` — ein absoluter Pfad in `sources:` gibt seine Wurzel
**nicht** selbst frei.

## Schritt 4 — Quellenkonfiguration ergänzen

Nur wenn Schritt 3 Quellen ergeben hat **und** der Nutzer Frage 3 bejaht hat, oder wenn er
auf Frage 4 Bereiche genannt hat. Für `exclude:` gilt dieselbe Schreibregel wie für
`sources:` — die Liste unten macht dabei keinen Unterschied zwischen beiden.

Die Datei ist handgepflegt. Für sie gilt die Schreibregel des Vertrags, und sie gilt für
jeden Aufrufweg gleich:

- **Nur nach ausdrücklicher Bestätigung.** Ohne sie wird nicht geschrieben. Ein „ja" auf
  Frage 3 aus Schritt 3 ist diese Bestätigung nur zusammen mit der Diff-Bestätigung unten.
- **Nur ergänzend.** Neue Einträge kommen ans Ende der jeweiligen Liste.
- **Bestehende Einträge, Kommentare und Reihenfolge bleiben unangetastet.** Nichts wird
  umsortiert, neu formatiert, zusammengefasst oder von Kommentaren befreit. Die
  auskommentierten Beispiele der Vorlage bleiben stehen.
- Fehlt die Datei, wird sie hier **nicht** angelegt: `/k-gui` anbieten (Schritt 1).

Zeige den Diff und lass ihn bestätigen, bevor du schreibst. Bei „nein" bleibt die Datei
unverändert; die genannten Quellen fließen dann in **diesen** Lauf nicht ein, und das wird
im Abschluss so gesagt.

## Schritt 5 — Erhebung anstoßen

```bash
k-playbook inventory
```

Aufgerufen aus dem Hauptverzeichnis, ohne weitere Argumente. Das Kommando erhebt und
schreibt in einem Zug.

**Baue die Erhebung nicht nach.** Manifeste, Lockfiles, Dockerfiles, Compose-, Helm- und
CI-Dateien werden nicht von Hand gelesen, keine Version wird geschätzt, keine Zeile
ergänzt. Der Grund ist nicht Bequemlichkeit: die Vertrauensgrenze, die Normalisierung und
die Byte-Stabilitätsregel stehen an genau einer Stelle im Werkzeug, und eine zweite,
nachgebaute Erhebung wäre eine zweite Auslegung desselben Vertrags.

Die Ausgabe des Laufs trägt: ausgewertete Quellen, konfigurierte Zusatzquellen, Einträge,
Abweichungen (davon widersprüchliche), abgelehnte Quellen, nicht durchsuchte Quellen und
Hinweise, dazu jede Ablehnung, jeden greifenden Ausschluss und jeden Hinweis im Wortlaut.
Übernimm sie unverändert in den Abschluss; **keine Ablehnung** wird dabei weggelassen.

Die Datei selbst wird danach nicht nachbearbeitet. Fällt am Ergebnis etwas auf, ist das ein
Befund für den Erzeuger, keine Korrektur von Hand — beim nächsten Lauf wäre sie ohnehin
weg.

## Schritt 6 — Abschluss

Kompakte Zusammenfassung:

- Quellenkonfiguration: vorhanden / fehlt / defekt, und ob sie in diesem Lauf ergänzt wurde
  (mit der Zahl neuer Einträge) oder unangetastet blieb.
- Zahlen des Laufs: ausgewertete Quellen, konfigurierte Zusatzquellen, Einträge,
  Abweichungen (davon widersprüchliche), abgelehnte Quellen, nicht durchsuchte Quellen,
  Hinweise.
- `INVENTORY_DISPLAY_PATH`: neu geschrieben (mit Erhebungszeitpunkt) oder unverändert, weil
  die Erhebung inhaltlich dieselbe ist. Beides ist ein erfolgreicher Lauf.
- Jede abgelehnte Quelle mit angefragtem Pfad, aufgelöstem Pfad und Grund. Eine Ablehnung
  ist eine Lücke im Inventar; eine Lücke, die niemand sieht, ist schlimmer als ein Fehler.
- Ausdrücklich: außerhalb von `VERSIONS_DISPLAY_PATH` wurde nichts geschrieben — außer der
  bestätigten Ergänzung in `VERSION_SOURCES_DISPLAY_PATH`.
- Hinweis, falls Abweichungen der Art `widersprüchlich` stehen: dieselbe Umgebung sagt
  Verschiedenes. Das Inventar löst das nicht auf, es weist es aus.
- Jeden Ausschluss, der Quellen übergangen hat, mit Muster, Herkunft und Zahl. Ein
  Ausschluss ist keine Ablehnung — der Bereich dürfte gelesen werden —, aber eine Stelle,
  an der nicht gesucht wurde, und die gehört genauso in den Bericht. Kommen auffällig viele
  Abweichungen aus einem einzigen Verzeichnis mit Testmaterial, ist das der Anlass, Frage 4
  aus Schritt 3 zu stellen — nicht der Anlass, still zu filtern.
- Den Abgleich mit `docs/libs/` macht `/k-docs` in seinem Schritt 3; hier wird er nicht
  wiederholt.
- Folge-Command: **`/k-docs-index`** — nimmt die Inventardatei als Herkunft „Versionen" in
  den einzigen Index `k-playbook-local/docs/README.md` auf und verlinkt sie. Ohne diesen
  Lauf taucht sie dort nicht auf.

## Fehlerfälle

- `RESOLVED_DOCS_DIR` fehlt → fragen, ob genau dieses Verzeichnis angelegt werden soll, oder
  `/k-gui` nennen. Kein Ersatzpfad, kein harter Abbruch.
- `versionSources.error` ist gesetzt → die Quellenkonfiguration ist defekt. Der
  Erhebungslauf bricht damit ab und schreibt nichts; das Kommando meldet Datei, Zeile und
  Parser-Meldung. Die Meldung weitergeben und die Datei **nicht** selbst reparieren — eine
  defekte Konfiguration halb zu deuten hieße, eine andere Vertrauensgrenze anzuwenden als
  die aufgeschriebene.
- `k-playbook inventory` meldet `unbekanntes Kommando` → die Installation ist älter als
  dieser Command. Melden, Update oder `/k-gui` nennen, und die Erhebung **nicht** von Hand
  nachbauen.
- Das Kommando meldet abgelehnte Quellen → das ist kein Fehlschlag: der Lauf geht weiter,
  die Ablehnungen stehen im Inventar und im Abschluss. Sie zu übergehen ist der Fehler.
- `INVENTORY_FILE` ist da, ihr Frontmatter defekt → das Kommando meldet es als Befund und
  schreibt neu, weil ein Vergleich nicht möglich ist. Den Befund im Abschluss nennen.
- Der Nutzer bestätigt den Diff der Quellenkonfiguration nicht → Datei unverändert lassen,
  Erhebung trotzdem laufen lassen, im Abschluss als übersprungen führen.

## Anti-Muster (nicht tun)

- **Die Erhebung nachbauen.** Manifeste von Hand lesen, Versionen schätzen, Zeilen ergänzen
  — dann gäbe es zwei Auslegungen des Vertrags, und die zweite ginge in keine Prüfung ein.
- **`version-sources.yaml` selbst lesen.** Ihr Zustand steht in `versionSources`. Wer sie
  direkt öffnet, liest eine zweite, womöglich andere Antwort.
- **Die Quellenkonfiguration umschreiben.** Kein Umsortieren, kein Neuformatieren, kein
  Entfernen von Kommentaren, kein Schreiben ohne Bestätigung — sie ist handgepflegt.
- **Die Inventardatei von Hand nachbessern.** Sie wird bei jedem Lauf neu erzeugt; die
  Korrektur ist beim nächsten Mal weg und verdeckt bis dahin einen Befund.
- **Abweichungen auflösen.** Sie werden ausgewiesen, nicht auf einen „richtigen" Wert
  reduziert — das ist der Zweck des Inventars.
- **Ablehnungen still übergehen.** Eine konfigurierte Quelle, die nicht gelesen werden
  konnte, gehört sichtbar in den Abschluss.
- **Still filtern.** Ein Ausschluss wird vorgeschlagen und bestätigt, nie nebenbei
  eingetragen — und er steht danach im Inventar. Ein Bereich, der lautlos verschwindet, ist
  eine Lücke, die niemand sieht.
- **Den Index anfassen.** `docs/README.md` gehört `/k-docs-index`; eine hier eingefügte
  Zeile ist beim nächsten Lauf weg.
