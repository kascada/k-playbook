# Regel: Code und Docs synchron halten

## Grundsatz

Wenn Code geändert wird, muss geprüft werden, ob die projektinterne Dokumentation dadurch veraltet, unvollständig oder irreführend wird.

## Verpflichtung

Bei jeder Code-Änderung gilt:

- Relevante Docs müssen im selben Arbeitsgang angepasst werden.
- Wenn unklar ist, welche Docs betroffen sind, muss nach passenden Docs gesucht werden, bevor die Änderung abgeschlossen wird.
- Wenn weiterhin unklar ist, ob oder wie Docs angepasst werden sollen, muss der User gefragt werden.
- Wenn bewusst keine Doc-Änderung nötig ist, soll das im Abschluss kurz begründet werden.

## Was zählt als relevante Doku

Relevant sind insbesondere:

- Dateien unter `k-playbook-local/docs/code/` — die aus dem Code erzeugte Projekt-Doku. Sie folgt dem Code und veraltet mit ihm.
- Architektur-, Betriebs-, Setup-, API- und Datenmodell-Dokumentation.
- `README.md`, `AGENTS.md` oder andere Einstiegspunkte, wenn sie das geänderte Verhalten beschreiben.
- Projektinterne Guidelines oder Known-Decisions, wenn die Änderung eine Konvention oder bewusste Entscheidung berührt.

Geschrieben wird nur neben die Installation. Das Installationsverzeichnis selbst wird bei jedem Update vollständig ersetzt; Doku darin wäre beim nächsten Mal weg.

## Was von der Sync-Pflicht ausgenommen ist

Unter `k-playbook-local/docs/` hat jedes Unterverzeichnis eine eigene Herkunft, und nur eine davon folgt dem Code:

- `docs/libs/` folgt den Libraries des Projekts, nicht seinem Code. Eine Code-Änderung veraltet es nicht; ein Versionssprung einer Library schon.
- `docs/manual/` ist handgepflegt und folgt keinem Erzeuger. Wer sie ändern will, ändert sie bewusst.
- `docs/extracted/` hält fest, was aus Rohmaterial gewonnen wurde — ein Stand von damals, mit Quelle und Konfidenz. Er wird nicht nachgezogen, sonst verliert er seine Aussage.
- `docs/versions/` folgt den deklarierten Versionen des Projekts, nicht seinem Code. Eine Code-Änderung veraltet das Inventar nicht; eine geänderte Versionsangabe in einem Manifest, einer Container-, Helm- oder CI-Datei schon. Was ein solcher Versionssprung an Nachzug auslöst, steht im Abschnitt „Nachzug bei Versionssprüngen" unten und wird hier nicht ein zweites Mal ausformuliert.
- `docs/README.md` ist der erzeugte Index. Er wird nicht von Hand nachgezogen, sondern von `/k-docs-index` neu geschrieben.

Ändert sich dort tatsächlich etwas, ist der Weg der jeweilige Erzeuger — nicht die Handkorrektur im selben Arbeitsgang.

## Nachzug bei Versionssprüngen

Ein Versionssprung ist keine Code-Änderung; die Verpflichtung oben greift für ihn nicht. Er hat trotzdem eine Pflicht, und sie steht hier — nicht in einem Skill-Text und nicht als Erinnerung.

Ändert sich ein unterstütztes Manifest oder Lockfile — `pyproject.toml`, `requirements*.txt`, `go.mod`, `package.json`, `Cargo.toml`, `Gemfile`, ihre Lockfiles und die übrigen aus dem Vertrag —, eine Container-, DevContainer-, Compose-, Helm- oder CI-Datei, sodass eine Versionsangabe hinzukommt, wegfällt oder wechselt, dann veraltet `docs/versions/`, und das Inventar wird **im selben Arbeitsgang, über den Erzeuger** nachgezogen — nicht per Handkorrektur an `docs/versions/inventory.md`. Der Erzeuger ist auf jedem Weg derselbe Lauf:

- `/k-doc-inventory` im Assistenten (Modul `commands/_docs/inventory.md`),
- `k-playbook inventory` auf der Kommandozeile,
- der Bereich „Inventar" der Oberfläche (`k-playbook` ohne Argument, Knopf „Aktualisieren").

Alle drei rufen dieselbe Go-Fachlogik und schreiben dieselbe Datei. Ein Lauf ohne inhaltliche Änderung lässt sie byte-identisch stehen — nachziehen kostet nichts, wenn nichts zu ändern war, und es gibt keinen Grund, es zu unterlassen.

Zwei Fälle, die auseinanderzuhalten sind:

- **Die Änderung stammt aus dem laufenden Arbeitsgang.** Dann wird das Inventar im selben Arbeitsgang aktualisiert, und der Abschluss nennt das Ergebnis: geschrieben mit Erhebungszeitpunkt, oder unverändert, weil die Erhebung inhaltlich dieselbe ist.
- **Nur eine fremde, bereits vorhandene Abweichung wird entdeckt** — das Inventar ist älter als ein Manifest, das jemand anders geändert hat, oder es weist eine Abweichung der Art `widersprüchlich` aus. Dann ist das ein sichtbarer Hinweis an den Nutzer, keine stille Aktualisierung nebenbei: der Nachzug ist eine eigene, benannte Handlung, die er auslöst oder ablehnt.

Die Quellenkonfiguration `k-playbook-local/version-sources.yaml` folgt dabei der Schreibregel des Vertrags („Quellenkonfiguration" in `k-playbook/docs/versionsinventar.md`): nur nach ausdrücklicher Bestätigung, nur ergänzend, bestehende Einträge, Kommentare und Reihenfolge bleiben erhalten. Sie wird hier nicht neu und nicht strenger formuliert.

`docs/libs/` bleibt davon getrennt. Ein Versionssprung veraltet auch die Steckbriefe dort, nachgezogen werden sie aber über `/k-docs-tools`, nicht durch Umschreiben aus dem Inventar. Weichen beide voneinander ab, gibt das Inventar die Auskunft, und `/k-docs` meldet den Unterschied als Hinweis; die Abgrenzung steht in `commands/_docs/tools.md` und `commands/k-docs.md`.

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
- Es wurde geprüft und begründet, warum keine Doc-Änderung nötig ist.
- Der User hat entschieden, wie mit fehlender oder unklarer Doku umzugehen ist.
