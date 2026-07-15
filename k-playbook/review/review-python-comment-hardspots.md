---
name: review-python-comment-hardspots
title: Python — Nicht-rekonstruierbare Entscheidungen kommentieren
language: python
interval-weeks: 16
scope-hint: Python-Quellen; Ausschluss: virtuelle Umgebungen, tests/fixtures
---

# Review: Python-Comment-Hardspots

Finde in Python-Code Stellen, an denen **selbst ein erfahrener Reviewer mit Projektkenntnis nicht erkennen kann, warum der Code so geschrieben wurde wie er ist**. Kläre den Grund (notfalls per Rückfrage) und schlage dort einen aussagekräftigen Kommentar vor.

> Der generische Ablauf (Scan → Übersicht → gebündelte Rückfragen → Stelle-für-Stelle mit Freigabe → Log-Eintrag) wird von `/k-review` orchestriert. Diese Datei beschreibt nur die reviewspezifischen Kriterien, Stil-Wahl und Beispiele.

## Zielgruppe des Codes

Sehr erfahrene Programmierer, die das Projekt kennen. Konsequenzen:

- **Lokale Komplexität braucht keine Erklärung.** Bit-Manipulation, dichte Comprehensions, Regex, Algorithmen-Tricks – die werden gelesen und verstanden. Finger weg.
- **Domänenwissen ist vorausgesetzt.** Geschäftsregeln, Protokoll-Details, Spec-Verweise – alle kennen das. Nicht kommentieren.
- **Python-Idiome sowieso.** Auch ungewöhnliche, solange sie idiomatisch sind.

Es bleibt eine eng umrissene Restmenge: Stellen, an denen der Code aus Lesersicht *anders aussieht als die naheliegende Lösung* – und der Grund dafür nicht aus Code, Kontext oder Domänenwissen ableitbar ist.

## Was kommentiert werden soll

Konkret: Stellen, an denen die Antwort auf „warum nicht einfach so?" außerhalb des sichtbaren Universums liegt.

Typische Fälle:

- **Workarounds für externe Bugs/Quirks.** Library verhält sich nicht wie dokumentiert, OS-spezifisches Verhalten, Race-Condition in fremdem System, Browser-/Runtime-Eigenheiten.
- **Reihenfolge-Abhängigkeiten, die nicht aus dem Code folgen.** „Muss vor X passieren, weil sonst Y" – wenn die Begründung in einem ganz anderen Modul oder einer externen Komponente steckt.
- **Bewusste Abweichungen vom Naheliegenden.** Der Code macht es umständlich, weil die einfache Lösung in der Vergangenheit Probleme gemacht hat (Performance, Deadlock, Edge Case in Produktion).
- **Magische Werte, deren Ursprung nicht erschließbar ist.** `time.sleep(0.3)` – warum 0.3? Ein Timeout, der genau passen muss zu etwas Externem.
- **Unterdrückte Fehler mit nicht-offensichtlichem Grund.** `except SomeError: pass` – warum genau dieser Fehler hier okay ist, wenn man es nicht aus dem Kontext sieht.
- **Auskommentierter Code, der bleiben soll** – mit Grund warum (selten, aber kommt vor).
- **Nicht-portierbare oder versions-abhängige Konstrukte.** „Funktioniert nur mit Library-Version X, weil ..."
- **Verträge mit Code an anderer Stelle.** „Diese Methode muss thread-safe bleiben, weil sie aus dem Worker-Pool aufgerufen wird" – wenn der Aufrufer nicht in derselben Datei steht.

## Was nicht kommentiert werden soll

- **Alles, was ein erfahrener Reviewer aus dem Code selbst erschließen kann** – egal wie komplex es lokal aussieht.
- **Domänenlogik.** Auch wenn sie nicht-trivial ist.
- **Standard-Patterns**, auch ungewöhnliche.
- **Performance-Optimierungen, deren Wirkung offensichtlich ist** (z. B. Caching mit klar erkennbarem Zweck). Nur kommentieren, wenn die Optimierung *gegen-intuitiv* ist.
- **Sicherheitsmaßnahmen, die als solche erkennbar sind.** Kommentieren nur, wenn der Angriffsvektor nicht offensichtlich ist.

**Faustregel beim Scan:** Stelle dir vor, ein Senior-Entwickler im Team sieht die Stelle im Code-Review. Würde er fragen „warum ist das so?" – und wäre die Antwort nirgends im Repo zu finden? Dann ist es ein Kandidat. Bei allem anderen: nicht aufnehmen.

Sei **sehr** zurückhaltend. Bei begründetem Zweifel: nicht aufnehmen. In typischem Code dieser Art sind das oft nur wenige Stellen pro Datei, manchmal null. Eine leere Fundliste ist ein gültiges Ergebnis.

## Stil-Wahl beim Kommentar

- **Inline am Zeilenende** (`x = 0.3  # API drosselt sonst, siehe Ticket #1234`): für sehr kurze Hinweise an einer einzelnen Zeile/Konstante.
- **Einzeiliger `#`-Kommentar darüber**: wenn der Hinweis ≤80 Zeichen passt und sich auf 1–3 Zeilen bezieht.
- **Block-Kommentar (mehrzeilig) über dem Abschnitt**: wenn mehrere Sätze nötig sind oder ein längerer Abschnitt betroffen ist.
- **Docstring**: nur wenn die nicht-rekonstruierbare Entscheidung das Wesen der gesamten Funktion/Methode betrifft (selten).

## Kommentar-Qualitätskriterien

Der Kommentar soll genau die Information liefern, die im Code fehlt – nicht mehr:

- **Erklärt das *Warum*, das man nicht erschließen kann.** Nicht das *Was* (das sehen Reviewer), nicht das offensichtliche *Warum* (z. B. „wir nutzen einen Cache für Performance" – das sieht man).
- **Nennt die externe Ursache konkret.** „Workaround für Bug in Library X v2.4" ist gut. „Da gab es Probleme" ist schlecht.
- **Verweist auf Quellen wo möglich.** Ticket-IDs, Bug-Reports im Upstream-Tracker, RFC-Sections, interne Doku-Seiten – wenn vorhanden und vom User bestätigt.
- **Ist so kurz wie möglich.** Bei dieser Zielgruppe genügen oft 1–2 Sätze. Kein Tutorial.
- **Altert gut.** Vermeide „neu", „kürzlich", „vorläufig", konkrete Versionsnummern ohne Bezug – außer der User bestätigt sie als bewusst gewählt.
- **Steht in der Sprache des umgebenden Codes** (Variablen-/Funktionsnamen als Indikator – meist Englisch).

## Beispiele

**Gute Kommentare (Zielmuster):**

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

**Kommentare, die NICHT vorgeschlagen werden sollten:**

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

## Reviewspezifische Anti-Muster

- **Lokale Komplexität kommentieren.** Wenn du anfängst, Regex/Comprehensions/Algorithmen zu erklären, hast du die Zielgruppe vergessen. Stop und neu evaluieren.
- **Domänenwissen kommentieren.** Geschäftsregeln, Spec-Bezüge, Branchenstandards – kennen alle. Nicht aufnehmen.
- **Leere Liste vermeiden wollen.** Wenn nichts unter die Kriterien fällt, ist die Antwort „nichts gefunden" – nicht „ich finde irgendwas, damit ich was zu liefern habe".
- **Vermutungen als Fakten formulieren.** Falsche Begründungen sind schlimmer als keine, weil sie die Wartbarkeit aktiv verschlechtern.
- **Das *Was* erklären statt das *Warum*.** Wenn der Reviewer den Code lesen kann (kann er), brauchst du ihn nicht zu paraphrasieren.
