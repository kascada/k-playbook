---
name: ks-befunde
description: Use when analysing unfamiliar code, chasing a root cause, or verifying a thesis by test - so that what was found gets written down instead of living only in the session. Writes one document per finding into keph:/queue/findings/ while the work is going on, following k-playbook:/instructions/on-demand/befunde.md. Trigger keywords - "Fehlersuche", "Ursache", "warum funktioniert das", "wie hängt das zusammen", "herausgefunden", "Sackgasse", "analysieren", "debuggen".
---

# Skill: Befunde

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Skills stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


**Kurzfassung.** Sorgt dafür, dass Erkenntnisse aus Analyse und Fehlersuche **während**
der Arbeit festgehalten werden — nicht erst am Ende, wenn der Kontext womöglich schon
komprimiert ist und die Einzelheiten fehlen.

## Wann anwenden

Immer wenn eine Arbeit Wissen erzeugt, das die nächste Sitzung sonst neu erarbeiten müsste:

- Fremder Code wird gelesen, um zu verstehen, wie oder warum er funktioniert.
- Einer Ursache wird nachgegangen, ein Fehlerbild eingekreist.
- Eine These wird durch einen Testlauf belegt oder widerlegt.
- Ein Weg wird geprüft und verworfen — die Sackgasse ist selbst ein Befund.
- Nutzeraussagen wie „warum macht der das", „finde raus, wie", „woran liegt das".

Nicht anwenden bei reiner Umsetzung nach bekanntem Muster, bei Formatierungen und
überall dort, wo nichts herausgefunden, sondern nur ausgeführt wurde.

## Anweisung

Verbindlich ist `k-playbook:/instructions/on-demand/befunde.md`. Lies sie mit
`k_playbook_doc_read` (ersatzweise `k-playbook read <verweis>`), bevor du den ersten
Befund dieser Sitzung schreibst. Sie enthält Ablage, Kopf, Rumpf, das Widerlegen und was
gilt, wenn Kephalaion nicht erreichbar ist.

**Diese Anweisung wird gelesen und befolgt, nicht hier wiederholt.** Weicht der Skill von
ihr ab, gilt die Anweisung.

Wenn der Context-Aufruf fehlschlägt, ist das Verzeichnis kein k-playbook-Projekt; kurz
darauf hinweisen und ohne Befund-Erfassung weiterarbeiten.

## Arbeitsweise

1. **Sobald etwas belegt ist**, einen Befund anlegen: knapp, mit Befund, Beleg und
   Sicherheit. Nicht warten, bis das ganze Bild steht — ein einzelner belegter Punkt ist
   einen Befund wert.
2. **Sobald ein Problem erkannt und gelöst ist**, ausführlicher: zusätzlich die
   ursprüngliche Frage, der erkennbare Hintergrund und vor allem die **ausgeschlossenen
   Wege**.
3. Geschrieben wird **ohne Rückfrage**. Gemeldet wird in einer Zeile, welcher Befund
   angelegt wurde — nicht sein Inhalt.
4. **In der Antwort den Verweis nennen.** Eine Analyse, deren Befund nirgends auftaucht,
   gilt als nicht abgeschlossen.

Ein Befund ist ein eigenes Dokument und wird danach nicht mehr geändert. Verwandte Befunde
tragen dasselbe `thema`; angehängt wird nicht.

## Expliziter Abschluss

Am Ende einer Sitzung schließt der Command den Vorgang ab — er legt die Befunde der Sitzung
vor, trägt Fehlendes nach, fragt, was aus ihnen werden soll, und prüft den Docs-Nachzug:

`/k-danke`
