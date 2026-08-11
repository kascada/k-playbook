# PLAYBOOK: Overlay-Repo-Analyse

**Ziel:** Ein Projekt mit **Docker-Overlay-Pattern** vollständig verstehen
und dokumentieren – insbesondere die Frage beantworten „was ist wirklich
Custom-Code des Overlays vs. was kommt schon aus der Base?"

**Aufwand:** ~1–3 Stunden für ein Projekt mit 2–4 Repos (je nach Größe).
Der größte Zeitfresser ist gründliches Lesen der Base.

**Warum wichtig:** Overlay-Repos enthalten nur Delta-Dateien. Ohne die Base
zu kennen, entstehen falsche Docs (Overlay-Code wird als Eigenentwicklung
missverstanden, obwohl er zu 90 % aus der Base kommt).

---

## Docker-Overlay-Pattern (kurze Erinnerung)

Ein Overlay-Repo hat typisch diese Struktur:

```
overlay-repo/
├── Dockerfile               FROM ghcr.io/org/base:tag  +  COPY . .
├── docker-compose.yaml      minimal
├── README.md
└── <wenige Dateien>         nur die, die überschrieben werden sollen
```

Zur Build-Zeit werden die Dateien im Overlay-Repo an dieselbe Stelle im
Base-Image gelegt und überschreiben dort die Originale. Alles nicht
Überschriebene kommt aus der Base.

Klassische Beispiele:
- LibreChat-Branding-Overlays
- Custom-Konfigurationen von Off-the-shelf-Software
- Squad-spezifische Erweiterungen einer Firmen-Base

---

## Phase 1: Bestandsaufnahme

Erste Übersicht, ohne tief einzusteigen. Ziel: verstehen, wie viele Repos
zusammengehören und welche Rolle jedes hat.

### Kommandos

```bash
cd <workspace-root>
ls -la

# Für jedes Repo:
for repo in */; do
  echo "=== $repo ==="
  ls "$repo"
  cat "$repo/Dockerfile" 2>/dev/null | head -5
  echo ""
done
```

### Worauf achten

- Welche Repos gibt es? Bilden sie Paare (`*-ui` + `*-api`)?
- Welches Repo hat `FROM ghcr.io/…` (= Overlay)? Welches nicht (= Base)?
- Welche `docker-compose.yaml`-Services sind wo definiert?
- Gibt es bereits einen `docs/`-Ordner?

### Ergebnis

Ein grobes mentales Modell wie:
> „4 Repos, 2 Paare. Overlay-Repos bauen jeweils auf Base-Images auf, die
> ihrerseits als eigene Repos existieren."

---

## Phase 2: Overlay-Analyse

Für jedes Overlay-Repo eine ausführliche Analyse. **Am besten mit
Subagenten parallel**, weil das zeitaufwendig ist.

### Vorgehen mit Subagent

Ein Subagent pro Repo, gestartet parallel. Prompt-Template siehe
`vorlagen/subagent-prompt-overlay.md.template`.

Wichtige Instruktionen an den Subagenten:

- Nur analysieren was **tatsächlich im Repo** liegt, keine Annahmen über
  Base-Image-Inhalte
- Klar trennen: „Repo enthält X" vs. „Repo referenziert X aus Base"
- Konkrete Zahlen (Dateien, Zeilen)
- Bugs/Auffälligkeiten notieren

### Worauf im Ergebnis achten

**Warnsignale, dass Base-Analyse dringend nötig ist:**

- Overlay-Analyse enthält viele Sätze wie „vermutlich…", „laut Imports
  wird… genutzt", „im Base-Image muss X existieren"
- `Dockerfile` referenziert ein Base-Image, dessen Inhalt unklar ist
- Sub-Agent bezeichnet Overlay-Code als „handgeschriebene Implementierung
  von X" – das kann falsch sein, wenn X aus Base kommt

### Ergebnis dieser Phase

Zwischenstand-Docs, die aber noch mit „⚠️ noch nicht mit Base
verglichen" markiert sein sollten.

---

## Phase 3: Base beschaffen

Zwei Wege. Idealerweise beide.

### Weg A: Git-Repo clonen (bevorzugt)

```bash
mkdir -p _bases   # falls Snapshots existieren
git clone https://github.com/<org>/<base-repo>.git <base-repo>

# Auf den Tag/Commit auschecken, der im Overlay gepinnt ist
cd <base-repo>
git checkout <sha aus dem <timestamp>-<sha>-Tag>
```

**Vorteile:** vollständige Historie, Autoren, PRs, Issues.
**Voraussetzung:** Repo ist zugänglich.

### Weg B: Docker-Image extrahieren

```bash
IMG=ghcr.io/org/base:<tag-aus-overlay-dockerfile>
DEST=_bases/<name>-<tag>

docker pull "$IMG"
CID=$(docker create "$IMG")
mkdir -p "$DEST"
docker cp "$CID:/app" "$DEST/app"
docker rm "$CID"
```

**Vorteile:** exakt die Version, die im Overlay gepinnt ist.
**Nachteile:** keine Git-History.

**Extraktionspfad:** meistens `/app`. Bei Node/UI-Projekten evtl. auch
`/app/node_modules` (aber groß), bei Python evtl. `/usr/local/lib/python3.*`
für Site-Packages.

### Voraussetzung prüfen

```bash
# Docker läuft?
docker info | grep "Server Version"

# GHCR-Login für private Images?
docker login ghcr.io
```

Für public Images ist kein Login nötig.

### Verifikation Base-Repo == Base-Image

Wenn beide Wege verfügbar sind, sollten sie identisch sein:

```bash
diff -rq _bases/<name>-<tag>/app <base-repo> \
  | grep -v "Only in <base-repo>: .git"
```

Diff sollte leer sein. Wenn nicht: Base-Repo-HEAD ist andere Version als
im Overlay gepinnter Tag → im Repo den passenden SHA auschecken.

---

## Phase 4: Diff Base ↔ Overlay

Jetzt der eigentliche Erkenntnisgewinn: welche Dateien wurden wirklich
verändert, und um wie viel?

### Rezept 1: Datei-Liste – was unterscheidet sich?

```bash
BASE=<base-repo>       # oder _bases/<name>-<tag>/app
OVR=<overlay-repo>

diff -rq "$BASE/app" "$OVR/app" 2>&1 | head -50
```

Ausgabe zeigt:
- `Only in $OVR: xyz` → neu im Overlay
- `Only in $BASE: xyz` → nur in Base (nicht überschrieben)
- `Files differ` → in beiden aber unterschiedlich

### Rezept 2: Konkrete Änderungen einer Datei

```bash
diff -u "$BASE/app/pfad/datei.py" "$OVR/app/pfad/datei.py" | less
```

### Rezept 3: Zeilen-Statistik

```bash
diff "$BASE/app/pfad/datei.py" "$OVR/app/pfad/datei.py" \
  | awk '/^</{del++} /^>/{add++} END{
      print "Gelöschte Zeilen:", del;
      print "Hinzugefügte Zeilen:", add;
    }'
```

### Rezept 4: Neu/entfernte Funktionen (Python-Beispiel)

```bash
grep -E "^\s*(async )?def " "$BASE/app/pfad/datei.py" | sort > /tmp/base.txt
grep -E "^\s*(async )?def " "$OVR/app/pfad/datei.py"  | sort > /tmp/ovr.txt

echo "=== Neu im Overlay ==="
comm -23 /tmp/ovr.txt /tmp/base.txt

echo "=== Nur in Base (entfernt im Overlay) ==="
comm -13 /tmp/ovr.txt /tmp/base.txt
```

Für andere Sprachen: `grep` anpassen. JS: `^(export )?(async )?function`,
`^const \w+ = `. TS: dito. Go: `^func `. Rust: `^(pub )?fn `.

### Rezept 5: Git als Diff-Werkzeug (elegant)

```bash
cd <base-repo>
git checkout -b _tmp_diff_check   # nicht auf main arbeiten

cp -r ../<overlay-repo>/app/* app/

git status                # welche Dateien anders
git diff --stat           # Zeilen-Statistik pro Datei
git diff                  # voller Diff

# Zurück
git checkout main && git branch -D _tmp_diff_check
```

Dieser Trick simuliert 1:1 das Dockerfile-`COPY` und zeigt jede Änderung.

### Interpretation

Für jede unterschiedliche Datei fragen:

- **Wie groß ist die Änderung** (in Zeilen)?
- **Was wurde funktional geändert** (neue Methoden? Umbenannte? Gelöschte?)
- **Fixt der Overlay einen Base-Bug**?
- **Ändert er ein Feld-Mapping / Schema**?

Notiere: „Overlay ergänzt X um Feature Y (+N Zeilen)".

---

## Phase 5: Docs schreiben (bzw. korrigieren)

Jetzt die Erkenntnisse in strukturierte Docs überführen. Wenn nach Phase 2
schon Zwischen-Docs existieren, jetzt revidieren.

### Wichtigste Regel

**Sauber trennen zwischen „Base liefert" und „Overlay ergänzt".**

Nicht mehr:
> „Der Overlay implementiert eine RAG-Pipeline mit Azure AI Search."

Sondern:
> „Die Base enthält bereits eine vollständige RAG-Pipeline (615 Zeilen).
> Der Overlay ergänzt sie um Parent Document Retrieval (+297 Zeilen, 5 neue
> Methoden). Der Azure-AI-Search-Adapter wird nahezu unverändert übernommen
> (3 Zeilen Δ)."

### Empfohlene Doc-Struktur

Pro Repo eine Datei mit klarer Trennung. Beispiel für Overlay-Doc:

```markdown
## 4. Was wird überschrieben?

### 4.1 Datei A – X Zeilen Δ

Beschreibung + konkreter Diff-Ausschnitt

### 4.2 Datei B – Y Zeilen Δ

...

## 5. Was vom Base übernommen wird (unverändert)

- FastAPI-Bootstrap
- Routing
- Provider-Framework
- ...

## 6. Base vs. Overlay – Trennlinie

Tabelle mit Zeile "Was liefert Base" | "Was ergänzt Overlay"
```

Vorlage in `<playbook.dir>/skills/overlay-repo-analyse/vorlagen/overlay-doc.md.template`

### Alle Docs verlinken

In `docs/README.md` alle Doc-Dateien listen. Idealerweise anschließend
das Playbook `ks-ai-session-memory/` anwenden, damit die Docs für
Folge-Sessions verankert werden.

> **Pfad-Hinweis:** Projektlokale k-playbook-Dokumentation liegt fest unter
> `k-playbook-local/docs/`; alternative Docs-Pfade gibt es nicht.

---

## Troubleshooting

### Base-Repo nicht verfügbar (privat/kein Zugriff)

- Base-Image trotzdem versuchen zu pullen (evtl. public)
- Wenn beides nicht geht: Overlay-Doku klar mit „⚠️ Base-Analyse
  ausstehend" markieren
- Später nachziehen, sobald Zugriff da ist

### Base-Repo und Base-Image differieren

- Overlay-Dockerfile pinnt einen `<timestamp>-<sha>`-Tag
- Im Base-Repo den passenden SHA auschecken:
  ```bash
  git checkout <sha>
  ```
- Danach nochmal diffen

### Overlay-Repo hat keine Tests

Häufig bei Overlays. Kann man tun:

- Nur die Deltas testen (nicht die ganze Base-Funktionalität)
- Bei Python: Base-Module per `sys.modules`-Stub mocken, echte Overlay-
  Module testen (siehe CLARA `squad-cdt-chatbot-api/tests/`)

### Riesige Ausgabe bei `diff -rq`

Auf App-Verzeichnisse einschränken:

```bash
diff -rq "$BASE/app" "$OVR/app"      # statt kompletter Repo-Root
```

Oder Ausschluss von `node_modules`, `.git`:

```bash
diff -rq --exclude=node_modules --exclude=.git "$BASE" "$OVR"
```

---

## Checkliste zur Verifikation

- [ ] Alle Overlay-Repos in Phase 2 einzeln analysiert
- [ ] Für jedes Overlay identifiziert: Base-Image-Tag (aus `Dockerfile`)
- [ ] Base beschafft (via `git clone` oder `docker cp`)
- [ ] Base-Repo-Version und Base-Image-Version übereinstimmen (verifiziert)
- [ ] Für jedes überschriebene File ein Diff durchgeführt
- [ ] Zeilen-Deltas quantifiziert (± X Zeilen)
- [ ] Neu/entfernte Funktionen aufgelistet
- [ ] Bugs/Design-Brüche in Docs notiert
- [ ] Doc-Struktur trennt sauber Base-Verhalten von Overlay-Delta
- [ ] Falls Zwischen-Docs aus Phase 2 existierten: revidiert
- [ ] `_bases/`-Verzeichnis ggf. in `.gitignore` (falls Workspace unter Git)
- [ ] Anschließend Playbook `ks-ai-session-memory/` angewendet

---

## Verwandte Playbooks

- `ks-ai-session-memory/` – wendet man **nach** diesem Playbook an, damit
  die entstandenen Docs für Folge-Sessions als autoritativ verankert
  werden.
