# Task 001 — Review-Lenses für /k-review-loop

Erweitert `/k-review-loop` um optionale, fachliche Review-Perspektiven ("Lenses"), die per Frontmatter im zu prüfenden Task aktiviert werden. Der bisherige generische Critic bleibt Default.

> **Pfad-Konvention:** Alle Pfade in diesem Task beziehen sich auf die Repo-Root (`/home/kleist/dev/k-playbook`). Das Playbook liegt im Unterverzeichnis `k-playbook/` (also `k-playbook/review/…`, `k-playbook/tasks/…`), während `commands/` und `docs/` direkt auf Repo-Ebene liegen. Die Namensgleichheit von Repo-Wurzel und Playbook-Verzeichnis ist beabsichtigt, aber verwirrungsanfällig.

## Intent

Der bestehende Review-Loop (Critic · Editor · Moderator) soll fachliche Blickwinkel bekommen, ohne seine Eleganz zu verlieren. Personas sind reine Prompt-Templates, keine "Charaktere" — der Rollentausch bleibt im Task kodiert (vgl. `docs/k-playbook.md:381`).

- Default-Verhalten von `/k-review-loop` bleibt unverändert (keine Breaking Changes für bestehende Task-Files ohne Frontmatter)
- Lenses sind **opt-in** per Frontmatter `review-lens: [architect, security, ...]` im geprüften Task
- Mehrere aktivierte Lenses laufen in Step 3 **parallel** als separate Critic-Subagents; Moderator dedupliziert Findings vor Editor-Runde
- Editor-Rolle bleibt unverändert (eine Instanz, generisch); Moderator-Rolle bleibt strukturell gleich, bekommt aber **zusätzlich** eine Dedup-/Konsolidierungs-Aufgabe, wenn mehrere Lenses aktiv sind
- Mindestens **eine Lens** ist als lauffähiger Prototyp umgesetzt und dokumentiert
- Lens-Prompts liegen als separate, versionierte Templates vor (nicht inline im Command-File)

## Referenzen

- `commands/k-review-loop.md` — die zu erweiternde Command-Definition
- `docs/k-playbook.md:342` — Design-Details des Review-Loops (Critic/Editor/Moderator)
- `docs/k-playbook.md:358` — Tabelle der Perspektiv-Umkehrungen (Basis für die Lens-Auswahl)
- `docs/k-playbook.md:398` — Umsetzungs-Status, hier ergänzt sich der neue Skill

## Ziel

1. Frontmatter-Feld `review-lens: [<name>, ...]` im **zu reviewenden** Task-File einführen (nicht im Command).
2. `/k-review-loop` liest das Feld beim Einlesen der Files (Step 1) und aktiviert die entsprechenden Lens-Prompts in Step 3.
   - **Single-File-Modus** (`/k-review-loop <file>`): Frontmatter wird aus genau dieser Datei gelesen.
   - **Directory-Modus** (`/k-review-loop <dir>`): Frontmatter wird **pro Task-File individuell** gelesen; jedes File kann eigene Lenses aktivieren. Es gibt keine Verzeichnis-übergreifende Lens-Konfiguration.
   - Frontmatter wird ausschließlich aus dem/den Task-File(s) gelesen, **nicht** aus referenzierten Kontext-Files.
3. Lens-Prompt-Templates in einem eigenen Verzeichnis ablegen, z. B. `k-playbook/review/lenses/<name>.md`.
4. Mindestens **eine** Lens vollständig ausformulieren — Vorschlag: `architect` (Systemgrenzen, Kopplung, Konsistenz mit bestehender Architektur), da sie den größten Hebel bei nicht-trivialen Tasks hat.
5. Skeletons/Platzhalter für weitere Lenses anlegen: `security`, `qa`, `ops`, `performance`, `ux`.
6. `k-review-loop.md` um die neue Mechanik ergänzen (Step 1 erweitern, Step 3 lens-aware machen, Step 4 Deduplikation erwähnen).
7. **Fehlerverhalten** bei unbekanntem Lens-Namen: Warnung im Startup-Summary (Step 2) im Format `⚠ Unbekannte Lens: <name> — übersprungen`, die betroffene Lens wird geskippt. Bleibt danach **keine** Lens aktiv, fällt der Loop auf den generischen Default-Critic zurück (kein Abbruch — die geplante Review soll trotz Tippfehler laufen können; die Warnung sorgt für Sichtbarkeit).

## Kontext

- Aktuelle Rollen (`commands/k-review-loop.md:344`): Critic (High-Level-Kritik), Editor (fix/begründen), Moderator (routet/entscheidet). Bewusst **High-Level, keine Details** (`docs/k-playbook.md:345`).
- Prinzip aus `docs/k-playbook.md:381`: Rollentausch im Task, nicht in Persona-Beschreibung → Lenses sind **Prompt-Varianten**, keine parallelen Personen mit Beziehungen untereinander.
- Kostenprinzip aus `docs/k-playbook.md:388`: Ein Modell reicht, keine Consensus-Overheads → Lenses laufen parallel, aber Moderator konsolidiert **vor** teurem Editor-Turn.
- Fallback: Wenn `review-lens:` fehlt oder leer ist → aktueller generischer Critic (kein Regress).

## Zu bauen

**Neue Dateien:**
- `k-playbook/review/lenses/architect.md` — vollständig ausformulierte Lens (Prompt-Template)
- `k-playbook/review/lenses/security.md` — Skeleton mit Titel + Fokus-Bereichen, TODO für Prompt
- `k-playbook/review/lenses/qa.md` — Skeleton (Widerlegungs-Perspektive, Edge-Cases)
- `k-playbook/review/lenses/ops.md` — Skeleton (Deploy, Rollback, Prod-Impact)
- `k-playbook/review/lenses/performance.md` — Skeleton (Skalierung, N+1, Memory)
- `k-playbook/review/lenses/ux.md` — Skeleton (CLI/API/Fehlermeldungen)
- `k-playbook/review/lenses/README.md` — kurze Doku: Wie füge ich eine neue Lens hinzu? Format, Konventionen.

**Geänderte Dateien:**
- `commands/k-review-loop.md`:
  - Step 1: zusätzlich Frontmatter-Feld `review-lens` parsen
  - Step 2: Startup-Summary um Zeile `Lenses: <namen> | — (generisch)` erweitern
  - Step 3: wenn Lenses aktiv → statt einem Critic-Subagent **N parallele** Subagents, jeder mit dem Prompt aus `k-playbook/review/lenses/<name>.md` (der Standard-Critic-Prompt wird zur generischen "Default-Lens")
  - Step 4: Moderator-Deduplication ergänzen (gleiche Findings aus verschiedenen Lenses zusammenführen). **Editor-Sichtbarkeit:** Der Editor bekommt die **konsolidierte, deduplizierte Findings-Liste ohne Lens-Herkunft** (weniger Kontext-Ballast, Fokus auf das Was, nicht das Wer). Die Lens-Herkunft jedes Findings wird nur im Discussion-Log (Step 9) für Nachvollziehbarkeit festgehalten.
  - Step 9: Discussion-Log erweitern um `**Lenses:** <namen>` und pro Finding die Lens-Herkunft (z. B. `[architect]`, `[security]`, `[architect+qa]` bei dedupliziertem Doppelfund)

- `docs/k-playbook.md`:
  - Sektion "Umsetzung im k-playbook" (`docs/k-playbook.md:398`): Lenses-Erweiterung dokumentieren
  - Perspektiv-Tabelle (`docs/k-playbook.md:358`): optional Verweis auf konkrete Lens-Files

**Nicht Teil dieses Tasks (bewusst später):**
- Automatische Lens-Auswahl per Heuristik (Task-Typ → Lens-Set)
- Sequenzielle Rollen-Kette à la BMad (Analyst → Architect → QA)
- Persona-Verkettung mit gegenseitigem Kontext

---
## Review-Log (2026-07-12)

**Pfad:** ./k-playbook/tasks
**Intent:** — (inline im Task)
**Runden:** 2 (Critic-Runde 2 fand keine neuen Issues)

### Diskussion

- **Pfad-Verwirrung `k-playbook/review/lenses/…` (FEHLER-01):** Critic vermutete doppelte Verschachtelung, weil das Working Directory bereits `/home/kleist/dev/k-playbook` heißt. Moderator wies nach, dass die Repo-Wurzel und das Playbook-Unterverzeichnis absichtlich denselben Namen tragen (`k-playbook/review/…` ist konsistent mit K-PLAYBOOK.MD-Pfaden). Auflösung: keine Umstrukturierung, aber klärender Blockquote unter H1 ergänzt.
- **Widerspruch „Moderator-Rolle unverändert" vs. neue Dedup-Aufgabe (FEHLER-02):** Critic zeigte, dass Intent und Ziel §6 sich widersprechen — Moderator bekommt bei mehreren Lenses eine neue, nicht-triviale Verantwortung. Editor entschärfte die Formulierung: Editor-Rolle bleibt komplett unverändert, Moderator-Rolle bleibt strukturell gleich, erhält aber explizit Dedup als Zusatzaufgabe.
- **Frontmatter-Quelle unklar (FEHLEND-03):** Critic bemängelte, dass nicht spezifiziert war, aus welcher Datei `review-lens:` gelesen wird. Editor klärte: Single-File-Modus liest aus dem übergebenen File, Directory-Modus pro Task-File individuell, nie aus Referenz-/Kontext-Files.
- **Verhalten bei unbekannter Lens (FEHLEND-04):** Editor übernahm den Moderator-Vorschlag: Warnung im Startup-Summary + Skip der Lens, Fallback auf Default-Critic falls keine Lens übrig. Begründung: Tippfehler soll die Review nicht komplett verhindern.
- **Editor-Sichtbarkeit der Lens-Herkunft (FEHLEND-07):** Editor entschied, dass der Editor eine konsolidierte, herkunftslose Findings-Liste bekommt (Fokus auf Sache, nicht Autorität). Lens-Herkunft nur im Discussion-Log via Format `[architect+qa]` bei Dedup-Doppelfund.

### Moderator-Entscheidungen

- **FEHLER-01** wurde vom Moderator zurückgewiesen, weil die Repo-Struktur die kritisierten Pfade tatsächlich erfordert. Als Kompromiss wurde ein klärender Hinweis akzeptiert.
- **WARNUNG-05** (Skeletons als tote Artefakte), **WARNUNG-06** (Kein Cap für Lens-Zahl), **WARNUNG-08** (`review-lens` vs. Plural) wurden als nicht ausführungsblockierend eingestuft und übersprungen.

### Intent-Alignment

**Yes** — Der Task deckt alle Intent-Punkte ab: Default-Verhalten bleibt via Fallback unverändert; Opt-in via Frontmatter; parallele Lens-Critics mit Moderator-Dedup; Editor bleibt generisch; `architect` als lauffähiger Prototyp plus Skeletons; Lens-Prompts als separate Templates. Pfad-Konvention und Nicht-Ziele grenzen den Scope sauber ab.

### Geänderte Dateien

- `k-playbook/tasks/001-review-lenses.md`:
  - Pfad-Konvention als Blockquote unter H1 ergänzt (FEHLER-01)
  - Intent-Bullet „Editor- und Moderator-Rolle unverändert" umformuliert (FEHLER-02)
  - Ziel §2 um Single-File-/Directory-Modus-Klarstellung erweitert (FEHLEND-03)
  - Neuer Ziel §7 zum Fehlerverhalten bei unbekanntem Lens-Namen (FEHLEND-04)
  - Step-4-Bullet in „Zu bauen" um Editor-Sichtbarkeits-Regel erweitert, Step-9-Bullet um Herkunfts-Format (FEHLEND-07)

### Offen (nicht gefixt)

- Keine.
