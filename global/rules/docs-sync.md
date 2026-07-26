# Regel: Code und Docs synchron halten

## Grundsatz

Wenn Code geaendert wird, muss geprueft werden, ob die projektinterne Dokumentation dadurch veraltet, unvollstaendig oder irrefuehrend wird.

## Verpflichtung

Bei jeder Code-Aenderung gilt:

- Relevante Docs muessen im selben Arbeitsgang angepasst werden.
- Wenn unklar ist, welche Docs betroffen sind, muss nach passenden Docs gesucht werden, bevor die Aenderung abgeschlossen wird.
- Wenn weiterhin unklar ist, ob oder wie Docs angepasst werden sollen, muss der User gefragt werden.
- Wenn bewusst keine Doc-Aenderung noetig ist, soll das im Abschluss kurz begruendet werden.

## Was zaehlt als relevante Doku

Relevant sind insbesondere:

- Dateien unter dem in `K-PLAYBOOK.MD` registrierten `docs:`-Pfad.
- Architektur-, Betriebs-, Setup-, API- und Datenmodell-Dokumentation.
- `README.md`, `AGENTS.md` oder andere Einstiegspunkte, wenn sie das geaenderte Verhalten beschreiben.
- Projektinterne Guidelines oder Known-Decisions, wenn die Aenderung eine Konvention oder bewusste Entscheidung beruehrt.

## Nicht automatisch dokumentieren

Keine Doc-Aenderung ist normalerweise noetig bei:

- Reinen Formatierungen ohne Verhaltensaenderung.
- Tests, die nur bestehendes Verhalten absichern.
- Internen Refactorings ohne geaenderte oeffentliche Semantik, sofern keine Doku diese interne Struktur beschreibt.
- Experimentellen Zwischenschritten, solange die Aenderung nicht als Ergebnis bestehen bleibt.

## Rueckfrage-Regel

Im Zweifel nicht raten. Stelle eine kurze Frage.

## Pruefkriterium

Eine Aenderung gilt nur dann als vollstaendig abgeschlossen, wenn eines davon zutrifft:

- Code und relevante Docs wurden gemeinsam aktualisiert.
- Es wurde geprueft und begruendet, warum keine Doc-Aenderung noetig ist.
- Der User hat entschieden, wie mit fehlender oder unklarer Doku umzugehen ist.
