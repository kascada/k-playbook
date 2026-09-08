---
name: ks-befunde
description: Use when analysing unfamiliar code, chasing a root cause, or verifying a thesis by test - so that what was found gets written down instead of living only in the session. Applies the rule k-playbook/rules/befunde.md and appends findings to k-playbook-local/material/befunde/ while the work is going on. Trigger keywords - "Fehlersuche", "Ursache", "warum funktioniert das", "wie hängt das zusammen", "herausgefunden", "Sackgasse", "analysieren", "debuggen".
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

## Regelquelle

Verbindlich ist die Regel `befunde.md` aus `catalogs.rules` der Context-Ausgabe. Sie
enthält Anlässe, Ablageort, Dateiformat, die Anhänge- und die Konfliktregel.

**Diese Regel wird gelesen und befolgt, nicht hier wiederholt.** Weicht der Skill von ihr
ab, gilt die Regel.

Wenn der Context-Aufruf fehlschlägt, ist das Verzeichnis kein k-playbook-Projekt; kurz
darauf hinweisen und ohne Befund-Erfassung weiterarbeiten.

## Arbeitsweise

1. **Sobald etwas belegt ist**, knapp anhängen: Befund, Beleg, Sicherheit. Nicht warten,
   bis das ganze Bild steht — ein einzelner belegter Punkt ist ein Eintrag wert.
2. **Sobald ein Problem erkannt und gelöst ist**, ausführlicher: zusätzlich die
   ursprüngliche Frage, der erkennbare Hintergrund und vor allem die **ausgeschlossenen
   Wege**.
3. Geschrieben wird **ohne Rückfrage**. Gemeldet wird in einer Zeile, welche Datei
   ergänzt wurde — nicht ihr Inhalt.
4. **In der Antwort die geschriebene Datei nennen.** Eine Analyse, deren Befund nirgends
   auftaucht, gilt als nicht abgeschlossen.

Ein Befund gehört zu einem Thema, nicht zu einer Sitzung: Gibt es zum Thema schon eine
Datei, wird dort angehängt, statt eine zweite anzulegen.

## Zwei Sorten, die woanders hingehören

In derselben Arbeit fällt regelmäßig Wissen an, das kein Projektbefund ist:

- **Betriebsfalle** — was man vor dem Handeln wissen muss („dieses Verzeichnis ist
  gesperrt", „dieser Test sagt nichts aus"). Ziel: `k-playbook-local/guidelines/betrieb.md`.
- **Prüfbare Regel** — was jedes Mal zu geschehen hat. Ziel: `k-playbook-local/rules/`.

Beides wird **nur nach ausdrücklicher Bestätigung** geschrieben; jeder Eintrag dort wirkt
dauerhaft. Im Zweifel bis zum Abschluss der Sitzung sammeln und dort fragen.

## Expliziter Abschluss

Am Ende einer Sitzung schließt der Command den Vorgang ab — er legt die Einträge vor,
befördert Bestätigtes in die Doku und zieht die Docs nach:

`/k-danke`
