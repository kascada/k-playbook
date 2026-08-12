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

- Dateien unter `<projekt>/k-playbook/docs/`.
- Architektur-, Betriebs-, Setup-, API- und Datenmodell-Dokumentation.
- `README.md`, `AGENTS.md` oder andere Einstiegspunkte, wenn sie das geänderte Verhalten beschreiben.
- Projektinterne Guidelines oder Known-Decisions, wenn die Änderung eine Konvention oder bewusste Entscheidung berührt.

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
