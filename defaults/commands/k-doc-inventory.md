# k-doc-inventory

Erhebt das Versionsinventar des Projekts: eine vollständige, reproduzierbare Übersicht der
**deklarierten** Versionen — Pakete, Tools, Runtimes, Container-Images, Helm-Charts und
Helm-Abhängigkeiten —, nach Umgebung getrennt und mit Herkunft je Zeile. Die Erhebung selbst
ist Go-Fachlogik: dieser Command klärt die Quellenlage, stößt das Subkommando
`k-playbook inventory` an und berichtet dessen Ergebnis. Er baut keine eigene Parserlogik
nach und nimmt kein Argument.

Der verbindliche Vertrag steht in `docs/version-inventory.md` der Installation: Datenmodell,
Pin-Taxonomie, Quellenarten, Abweichungen, Vertrauensgrenze, Quellenkonfiguration und die
Regel zu Zeitstempel und Byte-Stabilität. Dieser Text formuliert nichts davon neu.

Produces:

- `keph:/docs/generated/versions/inventory.md` — das Inventar, geschrieben vom Subkommando
  in die Collection des Projekts, nicht von Hand. Ein Lauf ohne inhaltliche Änderung lässt
  das Dokument byte-identisch stehen, Zeitstempel und Revision eingeschlossen.
- `k-playbook-local/version-sources.yaml` — **nur** nach ausdrücklicher Bestätigung des
  Nutzers und ausschließlich ergänzend (Schritt 4).

Nichts sonst.

## Schritt 1 — Briefing und Ablage

include: _shared/erster-schritt.md

include: _shared/docs.md

Das Inventar ist der Erzeuger `versions`: sein Verzeichnis ist
`keph:/docs/generated/versions/`, sein Dokument `keph:/docs/generated/versions/inventory.md`.
Der Name steht im Werkzeug; dieser Text setzt ihn nicht zusammen.

Aus dem Briefing brauchst du:

- `context.kephalaion` — `ok`, `address`, `message`. Ohne `ok: true` endet der Command hier
  (siehe die Ablage oben); `k-playbook inventory` bräche ohnehin mit
  `kephalaion_unavailable` ab.
- `context.versionSources` — der Zustand der Quellenkonfiguration
  `k-playbook-local/version-sources.yaml`. Lies die Datei **nicht** selbst — dieselbe Regel wie
  für `K-PLAYBOOK.yaml`.

## Schritt 2 — Quellenlage sichten

Zwei Auskünfte, beide ohne eigene Dateilektüre:

1. **Die Quellenkonfiguration** aus `context.versionSources`. Drei Zustände, und mehr nicht:
   - `present: true`, `error` leer → gültig. `roots`, `sources`, `exclude` und `helmValues`
     tragen den Inhalt; leere Listen heißen „nichts konfiguriert“. `schemaVersion` ist `1`
     oder `2`; `helmValues` wirkt nur unter `2`.
   - `present: false` → fehlt. Es gelten die Standardquellen unterhalb der Projektwurzel.
   - `error` gefüllt → defekt. Der Erhebungslauf bricht damit ab (Fehlerfälle unten).
   Fehlt das Feld `versionSources` ganz, ist die Installation älter als es: das sagen und auf
   `/k-upgrade` verweisen, statt die Datei ersatzweise selbst zu öffnen.
2. **Der Bestand des Inventars**: lies `keph:/docs/generated/versions/inventory.md` mit
   `k_playbook_doc_read` (ersatzweise `k-playbook read`). Kommt `not_found`, fehlt das
   Inventar — der Normalzustand vor dem ersten Lauf. Sonst nimm **nur** den YAML-Block
   zwischen den beiden `---`-Zeilen am Anfang — `generated.at` und die Zahlen unter
   `inventory:`. Der Markdown-Rumpf wird nie geparst.

Zeige die Lage kompakt:

```text
/k-doc-inventory — Quellenlage
─────────────────────────────────────
Collection    <context.kephalaion.address>
Inventar      erhoben <generated.at>, <N> Quellen, <N> Abweichungen | fehlt
Quellkonfig   vorhanden (<N> Wurzeln, <N> Quellen, <N> Ausschlüsse, <N> Helm-Werte) | fehlt | defekt: <error>
```

## Schritt 3 — Weitere Quellen erfragen

Nur beim ersten Lauf — also wenn das Inventar fehlt — oder wenn die Quellenkonfiguration
fehlt beziehungsweise keine `sources` führt. Sonst diesen Schritt überspringen: ein Command,
der bei jedem Lauf dieselbe Frage stellt, wird weggeklickt.

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
5. Stehen in Helm-values Versionen, die keine Image-Referenz sind — etwa ein Tag, den
   ein Subchart entgegennimmt? Dann brauchen sie Datei, Schlüsselpfad und Gegenstand
   (`container/<name>`).

Ohne Angabe läuft die Erhebung mit den Standardquellen; das ist eine gültige
Antwort und kein Mangel.
```

Die Installation `k-playbook/` ist immer ausgenommen und gehört nicht in Frage 4 — das
regelt der Vertrag, nicht die Konfiguration.

Nennt der Nutzer auf Frage 5 Werte, ordne jedem `path`, `key` und `item` zu, wie sie der
Vertrag unter „Configured Helm values“ beschreibt; der Kontext der Zeile kommt aus der
Quelle, als die die Datei gelesen wird, nicht aus dem Eintrag. Rate keinen Imagenamen:
fehlt er, frag danach.

Antwortet der Nutzer mit Quellen, ordne jeder eine Quellart (`kind`) und ein Umgebungslabel
(`env`) aus den geschlossenen Mengen des Vertrags zu und zeige die Zuordnung, bevor
irgendetwas geschrieben wird. Liegt eine Quelle außerhalb der Projektwurzel, braucht sie
zusätzlich eine Wurzel unter `roots:` — ein absoluter Pfad in `sources:` gibt seine Wurzel
**nicht** selbst frei.

## Schritt 4 — Quellenkonfiguration ergänzen

Nur wenn Schritt 3 Quellen ergeben hat **und** der Nutzer Frage 3 bejaht hat, oder wenn er
auf Frage 4 Bereiche oder auf Frage 5 Helm-Werte genannt hat. Für `exclude:` und
`helm_values:` gilt dieselbe Schreibregel wie für `sources:` — die Liste unten macht dabei
keinen Unterschied zwischen ihnen.

Die Datei ist handgepflegt. Für sie gilt die Schreibregel des Vertrags, und sie gilt für
jeden Aufrufweg gleich:

- **Nur nach ausdrücklicher Bestätigung.** Ohne sie wird nicht geschrieben. Ein „ja“ auf
  Frage 3 aus Schritt 3 ist diese Bestätigung nur zusammen mit der Diff-Bestätigung unten.
- **Nur ergänzend.** Neue Einträge kommen ans Ende der jeweiligen Liste. Die einzige
  Änderung an Bestehendem: Ist der neue Eintrag der erste unter `helm_values:` und steht
  `schema_version: 1` in der Datei, wird sie im selben Diff auf `2` gehoben — unter `1`
  lehnt das Werkzeug den Abschnitt ab.
- **Bestehende Einträge, Kommentare und Reihenfolge bleiben unangetastet.** Nichts wird
  umsortiert, neu formatiert, zusammengefasst oder von Kommentaren befreit. Die
  auskommentierten Beispiele der Vorlage bleiben stehen.
- Fehlt die Datei, wird sie hier **nicht** angelegt: `/k-gui` anbieten — es legt die gültige,
  leere Konfiguration aus der Vorlage des Werkzeugs an. Die Vorlage nicht selbst nachbauen.

Zeige den Diff und lass ihn bestätigen, bevor du schreibst. Bei „nein“ bleibt die Datei
unverändert; die genannten Quellen fließen dann in **diesen** Lauf nicht ein, und das wird
im Abschluss so gesagt.

## Schritt 5 — Erhebung anstoßen

```bash
k-playbook inventory
```

Aufgerufen aus dem Hauptverzeichnis, ohne weitere Argumente. Das Kommando erhebt und
schreibt in einem Zug in die Collection: Es liest den Bestand, vergleicht bis auf
`generated.at`, legt das Dokument an oder schreibt es mit der gelesenen Revision neu.

**Baue die Erhebung nicht nach.** Manifeste, Lockfiles, Dockerfiles, Compose-, Helm- und
CI-Dateien werden nicht von Hand gelesen, keine Version wird geschätzt, keine Zeile
ergänzt. Der Grund ist nicht Bequemlichkeit: die Vertrauensgrenze, die Normalisierung und
die Byte-Stabilitätsregel stehen an genau einer Stelle im Werkzeug, und eine zweite,
nachgebaute Erhebung wäre eine zweite Auslegung desselben Vertrags.

Die Ausgabe des Laufs trägt: ausgewertete Quellen, konfigurierte Zusatzquellen, Einträge,
Abweichungen (davon widersprüchliche), abgelehnte Quellen, nicht durchsuchte Quellen, nicht
auswertbare Quellen und Hinweise, dazu jede Ablehnung, jeden greifenden Ausschluss, jede
nicht auswertbare Quelle und jeden Hinweis im Wortlaut, und am Ende `Geschrieben:` oder
`Unverändert:` mit dem `keph:`-Verweis, dem Erhebungszeitpunkt und der Revision.
Übernimm sie unverändert in den Abschluss; **keine Ablehnung** wird dabei weggelassen.

Das Dokument selbst wird danach nicht nachbearbeitet. Fällt am Ergebnis etwas auf, ist das
ein Befund für den Erzeuger, keine Korrektur von Hand — beim nächsten Lauf wäre sie ohnehin
weg.

## Schritt 6 — Abschluss

Kompakte Zusammenfassung:

- Quellenkonfiguration: vorhanden / fehlt / defekt, und ob sie in diesem Lauf ergänzt wurde
  (mit der Zahl neuer Einträge) oder unangetastet blieb.
- Zahlen des Laufs: ausgewertete Quellen, konfigurierte Zusatzquellen, Einträge,
  Abweichungen (davon widersprüchliche), abgelehnte Quellen, nicht durchsuchte Quellen,
  nicht auswertbare Quellen, Hinweise.
- Jede nicht auswertbare Quelle mit ihrem Hinweis. Sie wurde gelesen, hat aber als Ganzes
  nichts ergeben; ohne diese Nennung sähe sie aus wie eine geprüfte Quelle ohne Fundstellen.
- `keph:/docs/generated/versions/inventory.md`: neu geschrieben (mit Erhebungszeitpunkt und
  Revision) oder unverändert, weil die Erhebung inhaltlich dieselbe ist. Beides ist ein
  erfolgreicher Lauf.
- Jede abgelehnte Quelle mit angefragtem Pfad, aufgelöstem Pfad und Grund. Eine Ablehnung
  ist eine Lücke im Inventar; eine Lücke, die niemand sieht, ist schlimmer als ein Fehler.
- Ausdrücklich: außerhalb von `keph:/docs/generated/versions/` wurde nichts geschrieben —
  außer der bestätigten Ergänzung in `k-playbook-local/version-sources.yaml`.
- Hinweis, falls Abweichungen der Art `widersprüchlich` stehen: dieselbe Umgebung sagt
  Verschiedenes. Das Inventar löst das nicht auf, es weist es aus.
- Jeden Ausschluss, der Quellen übergangen hat, mit Muster, Herkunft und Zahl. Ein
  Ausschluss ist keine Ablehnung — der Bereich dürfte gelesen werden —, aber eine Stelle,
  an der nicht gesucht wurde, und die gehört genauso in den Bericht. Kommen auffällig viele
  Abweichungen aus einem einzigen Verzeichnis mit Testmaterial, ist das der Anlass, Frage 4
  aus Schritt 3 zu stellen — nicht der Anlass, still zu filtern.
- Den Abgleich mit den Tool-Steckbriefen macht `/k-docs` in seinem Schritt 3; hier wird er
  nicht wiederholt.
- Folge-Command: keiner. Das Inventar liegt in `keph:/docs/`, und den Index darüber erzeugt
  Kephalaion, nicht ein Command.

## Fehlerfälle

- `context.kephalaion.ok` ist nicht `true` → Kephalaion ist nicht bereit. Nenne
  `context.kephalaion.message` und die Projekt-Collection auf der Setup-Seite (`/k-gui`).
  Keine Erhebung, kein Rückfall auf `k-playbook-local/docs/`. Meldet `k-playbook inventory`
  `kephalaion_unavailable`, gilt dasselbe.
- `k-playbook inventory` meldet `stale_revision` → zwischen Lesen und Schreiben hat jemand
  anders das Inventar geschrieben. Nichts wurde überschrieben. Den Lauf einmal wiederholen;
  meldet er es erneut, das sagen und nicht von Hand nachhelfen.
- `context.versionSources.error` ist gesetzt → die Quellenkonfiguration ist defekt. Der
  Erhebungslauf bricht damit ab und schreibt nichts; das Kommando meldet Datei, Zeile und
  Parser-Meldung. Die Meldung weitergeben und die Datei **nicht** selbst reparieren — eine
  defekte Konfiguration halb zu deuten hieße, eine andere Vertrauensgrenze anzuwenden als
  die aufgeschriebene.
- `k-playbook inventory` meldet `unbekanntes Kommando` → die Installation ist älter als
  dieser Command. Melden, `/k-upgrade` nennen, und die Erhebung **nicht** von Hand nachbauen.
- Das Kommando meldet abgelehnte Quellen → das ist kein Fehlschlag: der Lauf geht weiter,
  die Ablehnungen stehen im Inventar und im Abschluss. Sie zu übergehen ist der Fehler.
- Das Inventar ist da, sein Frontmatter defekt → das Kommando meldet es als Befund und
  schreibt neu, weil ein Vergleich nicht möglich ist. Den Befund im Abschluss nennen.
- Der Nutzer bestätigt den Diff der Quellenkonfiguration nicht → Datei unverändert lassen,
  Erhebung trotzdem laufen lassen, im Abschluss als übersprungen führen.

## Anti-Muster (nicht tun)

- **Die Erhebung nachbauen.** Manifeste von Hand lesen, Versionen schätzen, Zeilen ergänzen
  — dann gäbe es zwei Auslegungen des Vertrags, und die zweite ginge in keine Prüfung ein.
- **`version-sources.yaml` selbst lesen.** Ihr Zustand steht in `context.versionSources`.
  Wer sie direkt öffnet, liest eine zweite, womöglich andere Antwort.
- **Die Quellenkonfiguration umschreiben.** Kein Umsortieren, kein Neuformatieren, kein
  Entfernen von Kommentaren, kein Schreiben ohne Bestätigung — sie ist handgepflegt.
- **Das Inventar von Hand nachbessern** — weder in der Collection noch als Datei auf der
  Platte. Es wird bei jedem Lauf neu erzeugt; die Korrektur ist beim nächsten Mal weg und
  verdeckt bis dahin einen Befund.
- **Auf die Platte ausweichen.** Ohne Kephalaion gibt es kein Inventar. Eine Datei unter
  `k-playbook-local/docs/versions/` ist ein Stand von früher, kein gültiger Ort.
- **Abweichungen auflösen.** Sie werden ausgewiesen, nicht auf einen „richtigen“ Wert
  reduziert — das ist der Zweck des Inventars.
- **Ablehnungen still übergehen.** Eine konfigurierte Quelle, die nicht gelesen werden
  konnte, gehört sichtbar in den Abschluss.
- **Still filtern.** Ein Ausschluss wird vorgeschlagen und bestätigt, nie nebenbei
  eingetragen — und er steht danach im Inventar. Ein Bereich, der lautlos verschwindet, ist
  eine Lücke, die niemand sieht.
- **`README.md` oder `index.md` anfassen.** Sie erzeugt Kephalaion; eine hier eingefügte
  Zeile ist beim nächsten Erzeugen weg.
