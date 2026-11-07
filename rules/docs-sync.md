# Regel: Code und Docs synchron halten

## Grundsatz

Wenn Code geändert wird, muss geprüft werden, ob die projektinterne Dokumentation dadurch veraltet, unvollständig oder irreführend wird.

## Verpflichtung

Bei jeder Code-Änderung gilt:

- Relevante handgeschriebene Docs — Handdoku, `README.md`, `AGENTS.md` und andere Einstiegspunkte — müssen im selben Arbeitsgang angepasst werden.
- Für die erzeugte Doku, `keph:/docs/generated/code/` und `keph:/docs/generated/libs/` in der Projekt-Collection, ist der Normalfall, den Nachzug **einzureihen**: ein Doku-Auftrag in `keph:/queue/docs/` nach `k-playbook:/instructions/on-demand/doku-auftraege.md` — einer je Sitzung und Erzeuger, abgearbeitet gebündelt von `/k-docs-code` bzw. `/k-docs-tools`. Den Erzeuger sofort laufen zu lassen ist möglich; er schließt den Auftrag dann gleich mit ab. Wann nichts eingereiht wird (noch keine erzeugte Doku, kein Kephalaion), steht ebenfalls dort.
- Wenn unklar ist, welche Docs betroffen sind, muss nach passenden Docs gesucht werden, bevor die Änderung abgeschlossen wird.
- Wenn weiterhin unklar ist, ob oder wie Docs angepasst werden sollen, muss der User gefragt werden.
- Wenn bewusst keine Doc-Änderung nötig ist, soll das im Abschluss kurz begründet werden.

## Was zählt als relevante Doku

Relevant sind insbesondere:

- Die aus dem Code erzeugte Projekt-Doku, `keph:/docs/generated/code/` in der Projekt-Collection. Sie folgt dem Code und veraltet mit ihm; nachgezogen wird sie über `/k-docs-code`, im Normalfall über einen eingereihten Auftrag (`erzeuger: code`).
- Architektur-, Betriebs-, Setup-, API- und Datenmodell-Dokumentation.
- `README.md`, `AGENTS.md` oder andere Einstiegspunkte, wenn sie das geänderte Verhalten beschreiben.
- Projektinterne Guidelines oder Known-Decisions, wenn die Änderung eine Konvention oder bewusste Entscheidung berührt.

Geschrieben wird nur neben die Installation. Das Installationsverzeichnis selbst wird bei jedem Update vollständig ersetzt; Doku darin wäre beim nächsten Mal weg.

## Was von der Sync-Pflicht ausgenommen ist

Unter `keph:/docs/` hat jedes Unterverzeichnis eine eigene Herkunft, und nur eine davon folgt dem Code:

- `keph:/docs/generated/libs/` in der Projekt-Collection, die Steckbriefe von `/k-docs-tools`, folgt den Libraries des Projekts, nicht seinem Code. Eine Code-Änderung veraltet es nicht; ein Versionssprung einer Library schon — dann wird ein Auftrag mit `erzeuger: tools` eingereiht.
- Die handgeschriebene Doku, `keph:/docs/manual/` in der Projekt-Collection, folgt keinem Erzeuger. Wer sie ändern will, ändert sie bewusst. Ein `docs/manual/` auf der Platte ist ein Stand von vor dem Umzug; die Umstellung 3.2 in `/k-upgrade` bringt ihn nach Kephalaion.
- Ein `docs/extracted/` auf der Platte hält fest, was vor der Umstellung auf die Wissensablage aus Rohmaterial gewonnen wurde — ein Stand von damals, mit Quelle und Konfidenz. Er wird nicht nachgezogen, sonst verliert er seine Aussage.
- Das Versionsinventar, `keph:/docs/generated/versions/inventory.md` in der Projekt-Collection, folgt den deklarierten Versionen des Projekts, nicht seinem Code. Eine Code-Änderung veraltet das Inventar nicht; eine geänderte Versionsangabe in einem Manifest, einer Container-, Helm- oder CI-Datei schon. Was ein solcher Versionssprung an Nachzug auslöst, steht im Abschnitt „Nachzug bei Versionssprüngen" unten und wird hier nicht ein zweites Mal ausformuliert.
- Ein `docs/README.md` auf der Platte ist der alte Index von `/k-docs-index`; er wird nicht von Hand nachgezogen, `/k-upgrade` (3.2) räumt ihn ab.

Ändert sich dort tatsächlich etwas, ist der Weg der jeweilige Erzeuger — nicht die Handkorrektur im selben Arbeitsgang. Für die Steckbriefe heißt das im Normalfall: Auftrag einreihen; das Inventar wird sofort nachgezogen.

## Nachzug bei Versionssprüngen

Ein Versionssprung ist keine Code-Änderung; die Verpflichtung oben greift für ihn nicht. Er hat trotzdem eine Pflicht, und sie steht hier — nicht in einem Skill-Text und nicht als Erinnerung.

Ändert sich ein unterstütztes Manifest oder Lockfile — `pyproject.toml`, `requirements*.txt`, `go.mod`, `package.json`, `Cargo.toml`, `Gemfile`, ihre Lockfiles und die übrigen aus dem Vertrag —, eine Container-, DevContainer-, Compose-, Helm- oder CI-Datei, sodass eine Versionsangabe hinzukommt, wegfällt oder wechselt, dann veraltet das Inventar `keph:/docs/generated/versions/inventory.md`, und es wird **im selben Arbeitsgang, über den Erzeuger** nachgezogen — nicht per Handkorrektur am Dokument in der Collection und nicht an einer Datei unter `docs/versions/`. Der Erzeuger ist auf jedem Weg derselbe Lauf:

- `/k-doc-inventory` im Assistenten (Text `k-playbook:/commands/k-doc-inventory.md`),
- `k-playbook inventory` auf der Kommandozeile,
- der Bereich „Inventar" der Oberfläche (`k-playbook` ohne Argument, Knopf „Aktualisieren").

Alle drei rufen dieselbe Go-Fachlogik und schreiben dasselbe Dokument in der Collection. Ein Lauf ohne inhaltliche Änderung lässt es byte-identisch stehen — nachziehen kostet nichts, wenn nichts zu ändern war, und es gibt keinen Grund, es zu unterlassen.

Zwei Fälle, die auseinanderzuhalten sind:

- **Die Änderung stammt aus dem laufenden Arbeitsgang.** Dann wird das Inventar im selben Arbeitsgang aktualisiert, und der Abschluss nennt das Ergebnis: geschrieben mit Erhebungszeitpunkt, oder unverändert, weil die Erhebung inhaltlich dieselbe ist. Für die Steckbriefe wird ein Auftrag mit `erzeuger: tools` eingereiht.
- **Nur eine fremde, bereits vorhandene Abweichung wird entdeckt** — das Inventar ist älter als ein Manifest, das jemand anders geändert hat, oder es weist eine Abweichung der Art `widersprüchlich` aus. Dann ist das ein sichtbarer Hinweis an den Nutzer, keine stille Aktualisierung nebenbei: der Nachzug ist eine eigene, benannte Handlung, die er auslöst oder ablehnt.

Die Quellenkonfiguration `k-playbook-local/version-sources.yaml` folgt dabei der Schreibregel des Vertrags („Source Configuration" in `k-playbook/docs/version-inventory.md`): nur nach ausdrücklicher Bestätigung, nur ergänzend, bestehende Einträge, Kommentare und Reihenfolge bleiben erhalten. Sie wird hier nicht neu und nicht strenger formuliert.

Die Steckbriefe unter `keph:/docs/generated/libs/` bleiben davon getrennt. Ein Versionssprung veraltet auch sie, nachgezogen werden sie aber über `/k-docs-tools`, nicht durch Umschreiben aus dem Inventar und nicht sofort: Das Inventar wird sofort nachgezogen, die Steckbriefe werden eingereiht (Doku-Auftrag mit `erzeuger: tools`, `k-playbook:/instructions/on-demand/doku-auftraege.md`). Die Reihenfolge bleibt — zuerst das Inventar, danach die Steckbriefe, denn `/k-docs-tools` nimmt seine Kandidaten aus dem Inventar und nennt in `sources` die Versionen, gegen die gelesen wurde. Weichen beide voneinander ab, gibt das Inventar die Auskunft, und `/k-docs` meldet den Unterschied als Hinweis; die Abgrenzung steht im Vertrag des Inventars (`k-playbook/docs/version-inventory.md`) und im Text von `/k-docs-tools` (`k-playbook:/commands/k-docs-tools.md`).

## Nicht automatisch dokumentieren

Keine Doc-Änderung ist normalerweise nötig bei:

- Reinen Formatierungen ohne Verhaltensänderung.
- Tests, die nur bestehendes Verhalten absichern.
- Internen Refactorings ohne geänderte öffentliche Semantik, sofern keine Doku diese interne Struktur beschreibt.
- Experimentellen Zwischenschritten, solange die Änderung nicht als Ergebnis bestehen bleibt.

## Rückfrage-Regel

Im Zweifel nicht raten. Stelle eine kurze Frage.

## Prüfkriterium

Eine Änderung gilt nur dann als vollständig abgeschlossen, wenn eines davon zutrifft:

- Code und relevante Docs wurden gemeinsam aktualisiert.
- Code und handgeschriebene Docs wurden gemeinsam aktualisiert, und der Nachzug der erzeugten Doku ist eingereiht — Ausgang **„Nachzug eingereiht“**: Der Abschluss nennt den Verweis auf den Auftrag in `keph:/queue/docs/`, auch wenn es ein schon vorhandener Auftrag dieser Sitzung ist.
- Für die erzeugte Doku wurde bewusst nichts eingereiht, weil es sie noch nicht gibt oder Kephalaion nicht erreichbar ist; der Abschluss sagt das, im zweiten Fall mit dem fehlenden Nachzug als offenem Punkt.
- Es wurde geprüft und begründet, warum keine Doc-Änderung nötig ist.
- Der User hat entschieden, wie mit fehlender oder unklarer Doku umzugehen ist.
