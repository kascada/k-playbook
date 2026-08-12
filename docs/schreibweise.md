# Schreibweise

Alle deutschen Texte in diesem Repo verwenden korrekte Rechtschreibung mit Umlauten
und ß — keine ASCII-Umschreibung.

| statt | richtig |
|---|---|
| `fuer`, `ueber`, `pruefen` | für, über, prüfen |
| `Oberflaeche`, `Aenderung`, `naechste` | Oberfläche, Änderung, nächste |
| `heisst`, `schliessen`, `muessen` | heißt, schließen, müssen |

Das gilt für alles, was jemand liest: Dokumentation, Texte der Oberfläche, Commands,
Skills, Regeln, Review-Rezepte, Checks, Commit-Beschreibungen und Kommentare im Code.

## Wo ASCII bleibt

Nicht alles ist Text. Bei diesen Dingen bleibt es bei ASCII:

- **Datei- und Verzeichnisnamen**, und damit auch die Katalog-Schlüssel, die sich daraus
  ableiten. macOS speichert Umlaute zerlegt (NFD), Linux zusammengesetzt (NFC); derselbe
  Name wäre auf zwei Rechnern nicht mehr derselbe String. Der Overlay-Abgleich vergleicht
  genau diese Namen.
- **Konfigurationsschlüssel** in `K-PLAYBOOK.yaml`, Command- und Skill-Namen, Bezeichner
  im Code, Branch-Namen.

Kurz: was gelesen wird, bekommt Umlaute; was verglichen oder aufgerufen wird, nicht.

## Warum das unbedenklich ist

- Go-Quelldateien sind laut Sprachdefinition UTF-8.
- Die Oberfläche deklariert `charset=utf-8`, und der Dateiserver liefert für `.js` und
  `.css` denselben Charset mit.
- Markdown ist ohnehin UTF-8; die Doku benutzt längst `—`, `·` und `→`.

Der eigentliche Grund für die Festlegung ist ein anderer: **halb umgestellt ist der
schlechteste Zustand.** Solange beide Schreibweisen nebeneinander stehen, findet eine
Suche nach „Auflösung" die `Aufloesung`-Stellen nicht und umgekehrt.

## Beim Schreiben neuer Texte

Eine Suche nach `ae`, `oe`, `ue` ist unbrauchbar — die Folgen stecken auch in korrekten
Wörtern („neue", „Quelle", „aktuell", „Sequenz"). Wer prüfen will, sucht nach den
tatsächlichen Umschreibungen:

```bash
rg -i 'fuer|ueber|pruef|moeglich|naechst|waehl|aender|zurueck|oeffn|loesch|Oberflaeche|heisst|schliess|muess|koenn|laesst|traegt'
```
