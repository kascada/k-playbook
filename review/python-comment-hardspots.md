# Python: Nicht-rekonstruierbare Entscheidungen kommentieren

> **Vor dem Start:** [`known-decisions.md`](known-decisions.md) lesen — bereits dokumentierte Entscheidungen brauchen keinen weiteren Kommentar im Code.

Anweisung für Claude Code: Finde in Python-Code die Stellen, an denen **selbst ein erfahrener Reviewer mit Projektkenntnis nicht erkennen kann, warum der Code so geschrieben wurde wie er ist**. Kläre den Grund (notfalls per Rückfrage) und schlage dort einen aussagekräftigen Kommentar vor. Die Stellen werden einzeln im Chat besprochen.

## Wichtigste Regel

**Ändere den Code NICHT ohne explizite Freigabe pro Stelle.** Kein stillschweigendes Einfügen, kein Refactoring, keine Formatierung "nebenbei". Auch nicht zwei Stellen gleichzeitig anpacken. Pro Stelle: vorschlagen → Freigabe abwarten → einfügen → nächste Stelle.

## Zielgruppe des Codes

Sehr erfahrene Programmierer, die das Projekt kennen. Konsequenzen:

- **Lokale Komplexität braucht keine Erklärung.** Bit-Manipulation, dichte Comprehensions, Regex, Algorithmen-Tricks – die werden gelesen und verstanden. Finger weg.
- **Domänenwissen ist vorausgesetzt.** Geschäftsregeln, Protokoll-Details, Spec-Verweise – alle kennen das. Nicht kommentieren.
- **Python-Idiome sowieso.** Auch ungewöhnliche, solange sie idiomatisch sind.

Es bleibt eine eng umrissene Restmenge: Stellen, an denen der Code aus Lesersicht *anders aussieht als die naheliegende Lösung* – und der Grund dafür nicht aus Code, Kontext oder Domänenwissen ableitbar ist.

## Was kommentiert werden soll

Konkret: Stellen, an denen die Antwort auf "warum nicht einfach so?" außerhalb des sichtbaren Universums liegt.

Typische Fälle:

- **Workarounds für externe Bugs/Quirks.** Library verhält sich nicht wie dokumentiert, OS-spezifisches Verhalten, Race-Condition in fremdem System, Browser-/Runtime-Eigenheiten.
- **Reihenfolge-Abhängigkeiten, die nicht aus dem Code folgen.** "Muss vor X passieren, weil sonst Y" – wenn die Begründung in einem ganz anderen Modul oder einer externen Komponente steckt.
- **Bewusste Abweichungen vom Naheliegenden.** Der Code macht es umständlich, weil die einfache Lösung in der Vergangenheit Probleme gemacht hat (Performance, Deadlock, Edge Case in Produktion).
- **Magische Werte, deren Ursprung nicht erschließbar ist.** `time.sleep(0.3)` – warum 0.3? Ein Timeout, der genau passen muss zu etwas Externem.
- **Unterdrückte Fehler mit nicht-offensichtlichem Grund.** `except SomeError: pass` – warum genau dieser Fehler hier okay ist, wenn man es nicht aus dem Kontext sieht.
- **Auskommentierter Code, der bleiben soll** – mit Grund warum (selten, aber kommt vor).
- **Nicht-portierbare oder versions-abhängige Konstrukte.** "Funktioniert nur mit Library-Version X, weil ..."
- **Verträge mit Code an anderer Stelle.** "Diese Methode muss thread-safe bleiben, weil sie aus dem Worker-Pool aufgerufen wird" – wenn der Aufrufer nicht in derselben Datei steht.

## Was nicht kommentiert werden soll

- **Alles, was ein erfahrener Reviewer aus dem Code selbst erschließen kann** – egal wie komplex es lokal aussieht.
- **Domänenlogik.** Auch wenn sie nicht-trivial ist.
- **Standard-Patterns**, auch ungewöhnliche.
- **Performance-Optimierungen, deren Wirkung offensichtlich ist** (z. B. Caching mit klar erkennbarem Zweck). Nur kommentieren, wenn die Optimierung *gegen-intuitiv* ist.
- **Sicherheitsmaßnahmen, die als solche erkennbar sind.** Kommentieren nur, wenn der Angriffsvektor nicht offensichtlich ist.

**Faustregel beim Scan:** Stelle dir vor, ein Senior-Entwickler im Team sieht die Stelle im Code-Review. Würde er fragen "warum ist das so?" – und wäre die Antwort nirgends im Repo zu finden? Dann ist es ein Kandidat. Bei allem anderen: nicht aufnehmen.

## Workflow

### Schritt 1: Scan & Kandidatenliste

Lies die Datei(en) und identifiziere alle Stellen, die unter die Kriterien fallen. Erstelle eine interne Liste mit:
- Pfad + Zeilennummer(n)
- Vorläufige Einschätzung: Kannst du den Grund aus dem Repository (Kommentare anderswo, Tests, Commit-Messages falls einsehbar, Funktionsnamen, Importe) ableiten? (Ja / Teilweise / Nein)

Sei **sehr** zurückhaltend. Bei begründetem Zweifel: nicht aufnehmen. In typischem Code dieser Art sind das oft nur wenige Stellen pro Datei, manchmal null.

### Schritt 2: Übersicht zeigen

Bevor du in Details gehst, zeige eine kompakte Übersicht:

```
Ich habe N Stellen identifiziert, an denen der Grund für die Lösung
aus dem Code nicht erkennbar ist:

1. src/upload.py:67 – sleep(0.3) zwischen zwei Calls, Grund unklar
2. src/cache.py:142 – manuelles deepcopy statt direkter Zuweisung
3. src/api.py:88 – except ConnectionError wird geschluckt

Sollen wir die der Reihe nach durchgehen? Oder welche zuerst /
welche überspringen?
```

Falls die Liste leer ist: sag das ehrlich. *"Ich habe nichts gefunden, was unter die Kriterien fällt – der Code erklärt sich für die Zielgruppe selbst."* Das ist ein gültiges Ergebnis, kein Versagen.

### Schritt 3: Offene Fragen sammeln und am Stück stellen

Für Stellen, bei denen du den Grund nicht aus dem Repo ableiten kannst, **bündle alle Rückfragen in einer einzigen Nachricht**:

```
Bevor wir die Kommentare formulieren, ein paar Fragen zu den Gründen:

a) src/upload.py:67 – Das sleep(0.3): wartest du auf etwas Externes
   (API-Rate-Limit, Hardware-Reaktion)? Warum genau 0.3?

b) src/cache.py:142 – Das deepcopy statt direkter Zuweisung: gab es
   Probleme mit shared state, oder Mutationen flussabwärts?

c) src/api.py:88 – Wird der ConnectionError absichtlich geschluckt?
   Welches Szenario fängst du damit ab?
```

Bei den Antworten weiter zu Schritt 4. Wenn der User eine Frage nicht beantwortet oder das Wissen nicht hat: Hypothesen-Prinzip (siehe unten).

### Schritt 4: Stelle für Stelle besprechen

Für jede Stelle einzeln:

1. **Zeige die Stelle** mit ein paar Zeilen Kontext.
2. **Sag in einem Satz**, was die nicht-rekonstruierbare Entscheidung ist (nicht: dass der Code komplex sei – das ist nicht der Punkt).
3. **Schlage den Kommentar vor.** Stilwahl nach Heuristik:
   - **Inline am Zeilenende (`x = 0.3  # API drosselt sonst, siehe Ticket #1234`):** für sehr kurze Hinweise an einer einzelnen Zeile/Konstante
   - **Einzeiliger `#`-Kommentar darüber:** wenn der Hinweis ≤80 Zeichen passt und sich auf 1–3 Zeilen bezieht
   - **Block-Kommentar (mehrzeilig) über dem Abschnitt:** wenn mehrere Sätze nötig sind oder ein längerer Abschnitt betroffen ist
   - **Docstring:** nur wenn die nicht-rekonstruierbare Entscheidung das Wesen der gesamten Funktion/Methode betrifft (selten)
4. **Wenn du raten musstest**, kennzeichne das deutlich im Vorschlag (nicht im finalen Kommentar):
   ```
   Vorschlag (Hypothese – bitte bestätigen oder korrigieren):
   # Wartet, bis das Modem den AT-Befehl verarbeitet hat – ohne
   # Pause kommt manchmal eine ERROR-Antwort statt OK.
   ```
   Biete an: "Falls die Hypothese daneben ist, sag mir den richtigen Grund und ich formuliere um."
5. **Warte auf Freigabe.**
   - "Okay" / "Übernehmen" → Du fügst genau diesen Kommentar an genau dieser Stelle ein, nichts anderes.
   - "Änder das so: ..." → anpassen und erneut zeigen.
   - "Skip" / "Lass weg" → nichts einfügen, weiter.
6. **Nach dem Einfügen:** Diff/Snippet zeigen, dann nächste Stelle.

### Schritt 5: Abschluss

- Zusammenfassung: wieviele Kommentare eingefügt, wieviele geskippt.
- Falls dir beim Durcharbeiten weitere Kandidaten aufgefallen sind, die du zuerst übersehen hast: nennen, aber nicht eigenmächtig nachziehen.

## Kommentar-Qualitätskriterien

Der Kommentar soll genau die Information liefern, die im Code fehlt – nicht mehr:

- **Erklärt das *Warum*, das man nicht erschließen kann.** Nicht das *Was* (das sehen Reviewer), nicht das offensichtliche *Warum* (z. B. "wir nutzen einen Cache für Performance" – das sieht man).
- **Nennt die externe Ursache konkret.** "Workaround für Bug in Library X v2.4" ist gut. "Da gab es Probleme" ist schlecht.
- **Verweist auf Quellen wo möglich.** Ticket-IDs, Bug-Reports im Upstream-Tracker, RFC-Sections, interne Doku-Seiten – wenn vorhanden und vom User bestätigt.
- **Ist so kurz wie möglich.** Bei dieser Zielgruppe genügen oft 1–2 Sätze. Kein Tutorial.
- **Altert gut.** Vermeide "neu", "kürzlich", "vorläufig", konkrete Versionsnummern ohne Bezug – außer der User bestätigt sie als bewusst gewählt.
- **Steht in der Sprache des umgebenden Codes** (Variablen-/Funktionsnamen als Indikator – meist Englisch).

**Beispiele für gute Kommentare (Zielmuster):**

```python
time.sleep(0.3)  # Modem braucht Pause zw. AT-Cmds, sonst ERROR statt OK

# urllib3 wirft hier ConnectionError statt TimeoutError bei manchen
# Proxy-Konfigurationen – siehe urllib3#2754. Wir behandeln beides gleich.
except (ConnectionError, TimeoutError):
    ...

# Reihenfolge wichtig: _flush() muss VOR _close() laufen, weil der
# Writer im Worker-Pool sonst die Pipe schließt während noch geschrieben wird.
self._flush()
self._close()
```

**Beispiele für Kommentare, die NICHT vorgeschlagen werden sollten:**

```python
# Iteriere über alle Items                          # tautologisch
for item in items: ...

# Verwende List Comprehension für bessere Performance   # offensichtlich, Idiom
result = [x*2 for x in xs]

# Komplexer Regex                                   # lokal komplex, Zielgruppe versteht
re.compile(r'^(?P<ts>\d{4}-\d{2}-\d{2})T(?P<time>\d{2}:\d{2}:\d{2})')

# Berechne Rechnungsbetrag mit Mehrwertsteuer       # Domänenlogik, alle kennen das
total = net * (1 + VAT_RATE)
```

## Hypothesen-Prinzip

Wenn der User eine Rückfrage nicht beantworten kann oder will:

1. Formuliere deine beste Hypothese, basierend auf dem was im Repo erkennbar ist (andere Kommentare, Tests, Aufrufer, Bibliotheks-Kontext).
2. Kennzeichne sie **im Vorschlag** als Hypothese – **nicht im finalen Kommentar**.
3. Wenn der User bestätigt: Kommentar ohne Hypothese-Hinweis einfügen.
4. Wenn der User korrigiert: umformulieren, dann erneut Freigabe holen.
5. Wenn der User selbst keine sichere Antwort hat: gemeinsam überlegen, ob ein vorsichtig formulierter Kommentar (`# vermutlich ...`) besser ist als gar keiner, oder die Stelle skippen.

## Anti-Muster

- **Lokale Komplexität kommentieren.** Wenn du anfängst, Regex/Comprehensions/Algorithmen zu erklären, hast du die Zielgruppe vergessen. Stop und neu evaluieren.
- **Domänenwissen kommentieren.** Geschäftsregeln, Spec-Bezüge, Branchenstandards – kennen alle. Nicht aufnehmen.
- **Leere Liste vermeiden wollen.** Wenn nichts unter die Kriterien fällt, ist die Antwort "nichts gefunden" – nicht "ich finde irgendwas, damit ich was zu liefern habe".
- **Vermutungen als Fakten formulieren.** Falsche Begründungen sind schlimmer als keine, weil sie die Wartbarkeit aktiv verschlechtern.
- **Refactoring-Vorschläge einschmuggeln.** "Hier könnte man auch ..." gehört nicht hierher. Höchstens als kurze Schluss-Anmerkung.
- **Mehrere Stellen gleichzeitig editieren.** Pro Stelle: vorschlagen, Freigabe, einfügen, nächste.
- **Das *Was* erklären statt das *Warum*.** Wenn der Reviewer den Code lesen kann (kann er), brauchst du ihn nicht zu paraphrasieren.

## Log-Eintrag

Nach Abschluss in `priv/review/review-log.md` den Eintrag **Comment Hardspots** aktualisieren:
- `Letzter Lauf`: heutiges Datum (YYYY-MM-DD)
- `Fällig ab`: heutiges Datum + 16 Wochen

Außerdem eine Zeile ins Protokoll am Ende der Datei eintragen.
