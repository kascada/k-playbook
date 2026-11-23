---
title: Sperre des Arbeitsbaums — Sonderfälle
description: Modul für /k-task-run, vom Hauptkontext gelesen, wenn die Sperre nicht im Gutfall läuft — acquire meldet busy (Halter, Warten, Rückfrage bei unsicherem Halter), check meldet lost, die Marke ist unbekannt, oder das MCP-Werkzeug fehlt (Shell-Ersatz).
---

# Sperre des Arbeitsbaums — Sonderfälle

Dieses Modul liest der Hauptkontext von `/k-task-run`, wenn `acquire` nicht `acquired` meldet,
`check` nicht `held`, er `LOCK_TOKEN` nicht mehr kennt oder `k_playbook_lock` nicht verfügbar
ist. Was die Sperre ist und wann sie genommen, geprüft und freigegeben wird, steht im Command.

- **Halter.** `state: busy` nennt unter `holder` den Halter: `client`, `session`, `purpose`,
  `since`, `number` (die Nummer seiner Marke) und `orphaned` — `no`, `yes` oder `uncertain`,
  mit dem Grund in `orphanedWhy`. Einen sicher verwaisten Halter (`yes`) übernimmt `acquire`
  selbst und nennt ihn unter `takenOver`.

- **Ohne MCP-Werkzeug** gilt dasselbe über die Shell: `k-playbook lock <aktion> --json`
  mit `--token`, `--purpose`, `--session`, `--force`, `--expect` (Exit-Code 3 bei `busy`
  oder `lost`). So genommen kennt die Sperre keinen langlebigen Halter und gilt nie als
  sicher verwaist; endet die Sitzung ohne Freigabe, gibt der Nutzer mit
  `k-playbook lock release --force` frei.

## Acquire meldet busy

- `busy`: melden, ohne Rückfrage, und warten:

  ```
  Arbeitsbaum gesperrt von <client>, Sitzung <session>, wozu: <purpose>, seit <since>.
  Warte, bis die Sperre frei ist. Abbrechen jederzeit; freigeben nur, wenn der andere Lauf
  sicher nicht mehr arbeitet: k-playbook lock release --force
  ```

  Dann `wait` in Schleife (je Aufruf die Frist des Werkzeugs, ohne `seconds`), ohne
  Zeitgrenze, bis `acquired`. Ist `holder.orphaned` `uncertain`, kommt vorher eine
  Rückfrage, höchstens einmal je Halter, erkannt an `holder.number`: Fertig-Meldung
  (`Rückfrage: Sperre unsicher`), dann die Meldung oben mit `orphanedWhy` und der Frage
  „(a) warten / (b) freigeben und übernehmen / (c) abbrechen“. (a) wartet ohne weitere
  Rückfrage, bis frei; (b) ruft `release` mit `force` und `expect` = `holder.number`, dann
  wieder `acquire`; (c) beendet den Lauf ohne Sperre und mit der Fertig-Meldung
  `abgebrochen`. Die Wartezeit merkt sich der Hauptkontext für Schritt 7.

## Verloren

`state: lost` heißt: anhalten, nichts mehr am Arbeitsbaum ändern.
Fertig-Meldung (`Rückfrage: Sperre verloren`), dann:

```
Sperre verloren — <reason: freigegeben (released) | vom Nutzer freigegeben (forced) |
als verwaist übernommen (taken_over) | Sperrdatei fehlte (file_missing) | unbekannt>
Halter jetzt: <holder in einer Zeile | frei>
Seit dem letzten check kann noch geschrieben worden sein; ein laufender Sub-Agent wurde nicht
unterbrochen.

Wie weiter?
  (a) warten und neu nehmen
  (b) abbrechen
```

(a) nimmt neu wie unter „Sperre nehmen“ (Schritt 5), prüft danach die Liste neu und setzt am
Prüfpunkt fort; (b) verlässt Schritt 5 (Freigeben entfällt, die Sperre gehört dem Lauf nicht
mehr).

## Marke unbekannt

Kennt der Hauptkontext `LOCK_TOKEN` nicht mehr — etwa nach einer
Zusammenfassung des Kontexts —, ruft er `status` und fragt dann immer den Nutzer, gleich, wer
der Halter ist: Fertig-Meldung (`Rückfrage: Marke unbekannt`), die Frage nennt den Halter aus
`status` oder „frei“ und bietet (a) neu nehmen (warten, bis frei), (b) freigeben und neu
nehmen (`release` mit `force` und `expect` = `holder.number`) oder (c) abbrechen. Nie still
übernehmen oder neu nehmen.
