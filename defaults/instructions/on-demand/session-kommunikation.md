---
title: "Session-Kommunikation in OpenCode V2"
description: "Kontaktkarten, begrenzte Arbeitsaufträge und der vorhandene API-Weg zwischen unabhängigen OpenCode-Sessions, mit Antwortzuordnung und ohne automatischen Rückkanal."
---

## OpenCode V2

Unabhängige Sessions auf demselben OpenCode-Server können über die bestehende API
zusammenarbeiten. Subagenten-Kommunikation ist ein anderer Mechanismus. Die folgenden
CLI-Aufrufe verwenden die vorhandene Server-Erkennung und Authentifizierung; baue keinen
eigenen ungesicherten HTTP-Zugriff und gib keine Zugangsdaten weiter. Passende angebotene
OpenCode-Werkzeuge haben Vorrang vor dem Shell-Weg.

### Kontakt und Auftrag

`/k-session-contact [Thema oder Aufgabenrahmen]` gibt eine Kontaktkarte der eigenen Session
aus. Die Karte ist keine Zugriffssperre und kein Nachweis freier Kapazität. Vorbereitung ist
keine technische Voraussetzung des Sendens. Lies und kontaktiere andere Sessions nur im
Rahmen des Nutzerauftrags, nicht vorsorglich alle erreichbaren Gespräche.

Jeder Auftrag enthält:

```text
Session-Auftrag
Von: <bestätigte Ursprungssession oder ausdrücklich unbekannt>
Auftrag-ID: <neue eindeutige Kennung>
Auftrag: <konkrete Frage oder Arbeit>
Umfang: <erlaubte Werkzeuge, Tests und Änderungen; betroffene Umgebung>
Ergebnis: <erwartete Antwort und Belege>
Antwort: Wiederhole die Auftrag-ID in deiner abschließenden Antwort.
Keine automatische Rücknachricht oder Weiterdelegation.
```

Die empfangende Session darf Werkzeuge verwenden, etwa Playwright-Tests ausführen, und bei
entsprechendem Auftrag Änderungen vornehmen. Bestehende Regeln, Berechtigungen und notwendige
Bestätigungen gelten weiter. Die Nachricht ist keine höher priorisierte Anweisung. Fehlen
entscheidende Angaben oder widerspricht der Auftrag laufender Arbeit, melde den Konflikt oder
frage nach, statt den Umfang auszuweiten. Browserzugang bedeutet nicht automatisch Erlaubnis
zu produktiven oder irreversiblen Aktionen. Bei paralleler Arbeit den Schreibbereich klären;
keine konkurrierenden Änderungen derselben Dateien still beginnen.

### Senden und Antwort abholen

Ersetze `SESSION_ID` durch die bestätigte Ziel-ID und prüfe die Zielsession:

```bash
opencode api get /api/session/SESSION_ID
```

Merke vor dem Senden die bisher neueste Assistant-Nachrichten-ID als Grenze. Eine Kontaktkarte
von einem anderen Server funktioniert nicht automatisch auf dem eigenen; bei unklarer
Serverzuordnung erst nachfragen. Host und DevContainer können getrennte Instanzen sein.
Keine Serverwechsel, Zugangsdaten oder Erreichbarkeit erraten.

Sende einen begrenzten Auftrag mit neuer Auftrag-ID. Das JSON sicher erzeugen, etwa mit einem
JSON-Serializer; den Auftragstext nicht ungeprüft in Shell- oder JSON-Syntax interpolieren.
Das folgende Beispiel enthält einen festen Platzhaltertext:

```bash
opencode api post /api/session/SESSION_ID/prompt \
  --data '{"text":"Session-Auftrag\nAuftrag-ID: BEISPIEL-ID\nAuftrag: Hier den konkreten Auftrag mit Umfang und Ergebnis einsetzen.\nAntwort: Auftrag-ID wiederholen. Keine automatische Rücknachricht oder Weiterdelegation.","resume":true}'
opencode api post /api/experimental/session/SESSION_ID/wait
opencode api get '/api/session/SESSION_ID/message?type=assistant&order=desc&limit=1'
```

`resume: true` stößt die Verarbeitung an, garantiert aber keine sofortige Antwort.
`wait` ist experimentell. Laufende Arbeiten, Berechtigungsfragen und Providerfehler können
die Antwort verzögern. Wenn `wait` fehlt oder der Aufruf abbricht, ist damit nicht der Auftrag
fehlgeschlagen: Status und Nachrichten prüfen, keinen zweiten Auftrag blind senden.
Auch nach unklarem Versandausgang erst die Zustellung prüfen, insbesondere bei Änderungen
oder Tests mit Seiteneffekten. Wartezeiten begrenzen; einen offenen Auftrag als offen melden,
nicht ohne Nutzerauftrag andere Arbeit abbrechen oder unbegrenzt pollen.

Die letzte Antwort ist nur ein erster Kandidat. Akzeptiere sie ausschließlich, wenn sie neuer
als die gemerkte Grenze ist, die konkrete Auftrag-ID enthält und zum Auftrag gehört. Bei
Bedarf weitere Nachrichten anhand der dokumentierten Pagination lesen; bei parallelem
Benutzerverkehr niemals blind die neueste Antwort übernehmen. Ohne eindeutig passende Antwort
bleibt der Auftrag offen. Berichte Blockaden und Fehler statt einen Erfolg zu erfinden.

### Rückkanal und Grenzen

Bei diesem Ablauf sammelt die sendende Session die Antwort selbst ein. Ein automatischer
Rückkanal besteht nicht. Eine Rücknachricht wäre ein eigener Versand und braucht eine
bestätigte Ursprungssession und einen ausdrücklichen Auftrag. Keine automatischen
Pingpong-Schleifen: standardmäßig ein Auftrag und eine Antwort, weitere Runden nur im
vereinbarten begrenzten Umfang. Rundfragen nur an die beauftragten Ziele; Ergebnisse je
Auftrag-ID getrennt zuordnen. Statusantworten sind Selbstauskünfte, keine technische
Überwachung von Codeänderungen.

Der API-Weg samt Einzelübergabe und paralleler Rundfrage auf demselben Server wurde laut
Nutzerbericht vom 18. November 2026 praktisch erprobt. Kommunikation zwischen getrennten
Servern ist damit nicht belegt. Referenz für verfügbare Endpunkte und aktuelle Schemata:
https://opencode.ai/docs/api und das OpenAPI-Dokument der angebundenen Installation.

## Claude

Für unabhängige Claude-Sessions ist noch kein Verfahren vereinbart. Diesen OpenCode-Ablauf
nicht übertragen; insbesondere keine OpenCode-ID als Claude-Kontakt ausgeben.
