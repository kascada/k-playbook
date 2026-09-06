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
- `docs/versions/` folgt den deklarierten Versionen des Projekts, nicht seinem Code. Eine Code-Änderung veraltet das Inventar nicht; eine geänderte Versionsangabe in einem Manifest, einer Container-, Helm- oder CI-Datei schon. Was ein solcher Versionssprung an Nachzug auslöst, steht bei der Nachzugs-Pflicht bei Versionssprüngen und wird hier nicht ein zweites Mal ausformuliert.
- `docs/README.md` ist der erzeugte Index. Er wird nicht von Hand nachgezogen, sondern von `/k-docs-index` neu geschrieben.

Ändert sich dort tatsächlich etwas, ist der Weg der jeweilige Erzeuger — nicht die Handkorrektur im selben Arbeitsgang.

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
