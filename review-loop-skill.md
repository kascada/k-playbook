# Review-Loop — Skill-Dokumentation

Claude kann einen autonomen Review-Loop starten: ein Reviewer-Agent findet Fehler,
ein Author-Agent behebt sie, Claude moderiert dazwischen.
Dieser Loop läuft mehrere Runden bis zur Konvergenz.

---

## Wann anwenden

- Task-Beschreibungen, Konzepte, Architektur-Docs vor der Implementierung
- Spezifikationen die von jemandem erstellt wurden und von jemandem anderen geprüft werden sollen
- Alles wo "zweite Meinung + Iterieren" sinnvoll ist

Nicht geeignet für: Code-Review nach Implementierung (→ dafür `/review`).

---

## Rollen

| Rolle | Wer | Aufgabe |
|-------|-----|---------|
| **Reviewer** | Subagent (general-purpose) | Findet Fehler, Widersprüche, fehlende Details |
| **Author** | Subagent (general-purpose) | Behebt die gefundenen Fehler — nicht der Reviewer |
| **Moderator** | Claude (Hauptkontext) | Kategorisiert Issues, entscheidet was klar ist, eskaliert nur echte Fragen |

Wichtig: Author und Reviewer sind immer separate Agenten — derselbe Agent findet keine eigenen Fehler.

---

## Loop-Struktur

```
Runde 1:  Reviewer → findet Issues
Runde 2:  Moderator wertet aus → entscheidet FEHLER/WARNUNG/FEHLEND
          Author → behebt FEHLER (nicht Reviewer)
Runde 3:  Reviewer → prüft ob Korrekturen korrekt sind, neue Issues?
...
Stopp:    Reviewer meldet 0 FEHLER (WARNUNGEn und FEHLENDs sind ok)
Max:      3–4 Runden — danach eskaliert Moderator an User
```

---

## Reviewer-Prompt (Template)

Der Reviewer-Prompt muss enthalten:

1. **Kontext**: Was ist das System, wichtige Constraints, bestehende Architektur
2. **Material**: Alle zu prüfenden Dokumente (inline oder per Dateipfad)
3. **Prüfkategorien** (explizit vorgeben):
   - **FEHLER** — klar falsch, muss korrigiert werden
   - **WARNUNG** — möglicherweise problematisch oder unklar
   - **FEHLEND** — wichtiges Detail nicht spezifiziert
4. **Ausgabeformat** (vorgeben):
   ```
   | Kategorie | Task | Stelle | Problem | Empfehlung |
   ```
5. **Tonvorgabe**: "Sei präzise. Keine Einleitungen. Nur echte Probleme."

---

## Author-Prompt (Template)

Der Author-Prompt muss enthalten:

1. **Fehlerliste mit Moderator-Entscheidungen** — für jeden FEHLER: was genau zu ändern ist
2. **Originaltexte** aller zu korrigierenden Dokumente (vollständig)
3. **Explizite Anweisung**: "Schreib die korrigierten Dateien. Ändere NUR was durch die Fehlerliste begründet ist."
4. **Keine Zusammenfassung** — vollständige korrigierte Texte, keine Änderungsübersicht

Besser: Moderator trägt einfache Korrekturen selbst ein (schneller, kontrollierbar).

---

## Moderator-Aufgaben (Claude im Hauptkontext)

### Vor Author-Runde
Für jeden Issue entscheiden:

- **Klar falsch → fixen**: Technische Fehler die eine eindeutige richtige Antwort haben
- **Design-Entscheidung → ich entscheide**: z.B. "fail-fast oder lazy?", "no-op oder Exception?"
- **Widerspruch ohne klare Antwort → User fragen**: Wenn es eine Produktentscheidung ist, oder Fachkenntnis des Users nötig ist
- **WARNUNG/FEHLEND → abwägen**: Nur fixen wenn es für die Implementierung kritisch ist

### Wann an User eskalieren

Eskalieren wenn:
- Zwei Agenten haben denselben Punkt unterschiedlich bewertet und ich kann nicht entscheiden
- Eine Entscheidung hat Auswirkungen auf Produktfunktionalität (nicht nur technische Korrektheit)
- Mehr als 3 Runden ohne Konvergenz

### Entscheidungen dokumentieren

Jede nicht-triviale Entscheidung kurz im Chat begründen:
> "Entscheidung: Generator-Ende → reason='error' weil OpenAI den WebSocket nie aktiv schließt."

---

## Zweiter Review — Fokus-Prompt

Der zweite Reviewer-Prompt ist schlanker:
- Nur die geänderten Stellen zeigen
- Explizit fragen: "Hat die Korrektur neue Probleme eingeführt? Sind alle ursprünglichen Issues behoben?"
- Keine erneute Vollprüfung

---

## Kontext-Management

Bei großen Projekten wird der Kontext in Agenten-Prompts schnell groß.

**Alternativen:**
- Agenten lesen Dateien selbst per Tool (weniger Kontext im Prompt, dafür mehr Tool-Calls)
- Moderator gibt nur die relevanten Abschnitte weiter (selektives Einbetten)
- Bei sehr großen Projekten: Reviewer-Agent mit `subagent_type=Explore`

---

## Qualitätssicherung

**Was der Moderator nach Author-Runde prüft:**
- Hat der Author nur das geändert was in der Fehlerliste stand?
- Wurden die Dateien wirklich geschrieben (nicht nur eine Zusammenfassung geliefert)?
- Gibt es offensichtliche neue Probleme durch die Änderung?

**Konvergenz-Zeichen:**
- Reviewer: keine FEHLER mehr, nur noch WARNUNGEn und FEHLENDs
- Dieselben Issues tauchen in mehreren Runden auf → Moderator muss entscheiden

---

## Bekannte Verbesserungen (aus erster Anwendung)

1. Author-Prompt explizit: "Schreib die Dateien via Write-Tool — keine reine Zusammenfassung"
2. Issue-IDs durchgehend verwenden (FEHLER-01, FEHLER-02) — auch im Author-Prompt
3. Reviewer-Prompt: "Reviewe als Spezifikation, nicht als Code" (verhindert zu technische Implementierungsdetails als Warnings)
4. Maximale Runden immer definieren (Default: 3)
5. Moderator-Entscheidungen in separater Liste vor Author-Aufruf zusammenfassen
