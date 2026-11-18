# k-session-contact

Bereite die eigene Session als Kooperationspartner vor. Optionales Argument: Thema oder
Aufgabenrahmen. Kein Argument ist nötig; dann beschreibe den aktuellen Kontext knapp.

## Erster Schritt

Liegt das Briefing dieser Sitzung vor, verwende es; sonst rufe `k-playbook briefing` auf und
lies die Ausgabe. Lies `k-playbook:/instructions/on-demand/session-kommunikation.md`.

## Plattform prüfen

Dieses Verfahren gilt für OpenCode V2. In Claude melde kurz, dass das Verfahren für
unabhängige Claude-Sessions noch nicht definiert ist; keine OpenCode-Kontaktkarte erzeugen.

## Eigene Session bestimmen

Nutze eine verlässlich im Laufzeitkontext angebotene eigene OpenCode-Session-ID oder eine
geeignete Selbst-Auskunft eines angebotenen Werkzeugs. Die Kennung `session` aus dem
k-playbook-Briefing ist **keine** OpenCode-Session-ID. IDs von Subagenten sind nicht die eigene
ID. Keine Kennung erfinden, keine allein nach Titel, Arbeitsverzeichnis oder jüngstem
Zeitstempel ausgewählte Session als die eigene ausgeben.

Falls die eigene ID nicht zuverlässig zugänglich ist, frage den Nutzer nach der ID dieser
Session aus der Oberfläche oder einer bestätigten Zuordnung. Eine Kontaktkarte mit
`Session: unbekannt` darf als unvollständig ausgegeben werden, aber nicht als einsatzbereit.
Keine fremden Gesprächsinhalte zur Identifikation durchsuchen. Prüfe eine bestätigte ID mit
einem passenden OpenCode-Werkzeug oder `opencode api get /api/session/SESSION_ID` und gleiche
die verfügbaren Angaben ab. Bei Fehler oder Widerspruch keine Bereitschaft behaupten.

## Kontaktvereinbarung

Halte für diese Session fest: Sie nimmt im vereinbarten Rahmen konkrete Session-Aufträge
entgegen und beantwortet sie nach der On-demand-Anweisung. Werkzeuge, Playwright-Tests und
Änderungen sind entsprechend dem einzelnen Auftrag und bestehenden Berechtigungen erlaubt;
kein pauschales Verbot. Die Vorbereitung selbst startet keine Tests, ändert keine Dateien,
öffnet keinen Browser, nimmt keine frühere Arbeit wieder auf und sendet keine Nachricht an
andere Sessions. Keine Berechtigungen ändern und keine zusätzlichen Dienste starten.

Die Vereinbarung bleibt Gesprächskontext, keine dauerhafte Registrierung. Sie garantiert weder
freie Kapazität noch Empfang nach Verlust des Kontexts. Konflikte mit laufender Arbeit werden
bei Auftragseingang geklärt; der Nutzer kann den Rahmen jederzeit ändern oder aufheben.

## Kontaktkarte ausgeben

Gib einen kopierbaren Textblock aus:

```text
OpenCode-Session-Kontakt
Session: <bestätigte eigene ses_… oder unbekannt>
Erreichbarkeit: über denselben OpenCode-Server wie diese Session
Projekt/Arbeitsverzeichnis: <bekannter Pfad>
Thema/Aufgabenrahmen: <Argument oder knapper aktueller Kontext>
Fähigkeiten: <tatsächlich angebotene relevante Werkzeuge>
Aktuelle Arbeit: <knapper Stand und mögliche Überschneidungen>
Auftragsmodus: Konkreter Umfang je Auftrag; Werkzeuge und Änderungen nach Auftrag und Berechtigungen.
Antwortweg: Anfragende Session holt die Antwort mit Auftrag-ID ab; keine automatische Rücknachricht.
Status: <Kontakt vorbereitet oder unvollständig: eigene ID nicht bestätigt>
```

Keine Authentifizierungsdaten oder unnötigen Gesprächsinhalte aufnehmen. Ein vorhandenes
Playwright-Werkzeug belegt noch keine offene Anwendung oder Anmeldung: Solche Angaben nur
nennen, wenn sie aus dieser Session tatsächlich bekannt sind, sonst als ungeprüft kennzeichnen.
Die Serverzuordnung nicht aus einem Verzeichnisnamen ableiten; eine Karte ist zwischen
getrennten Host-/DevContainer-Instanzen nicht automatisch verwendbar.

Beende mit einem kurzen Hinweis: Die andere Session erhält die Karte und einen konkreten
Auftrag, etwa „Lass diese Session den Anmeldeablauf per Playwright prüfen“. Kein Probeversand
und keine Endlosschleife. Die Kontaktkarte ist eine Vereinbarung, keine technische Freigabe.
