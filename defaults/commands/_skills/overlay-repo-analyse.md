# Skill: Overlay-Repo-Analyse

**Ziel:** Ein Projekt mit **Docker-Overlay-Pattern** vollständig verstehen
und dokumentieren – insbesondere die Frage beantworten „was ist wirklich
Custom-Code des Overlays vs. was kommt schon aus der Base?"

**Aufwand:** ~1–3 Stunden für ein Projekt mit 2–4 Repos (je nach Größe).
Der größte Zeitfresser ist gründliches Lesen der Base.

**Warum wichtig:** Overlay-Repos enthalten nur Delta-Dateien. Ohne die Base
zu kennen, entstehen falsche Docs (Overlay-Code wird als Eigenentwicklung
missverstanden, obwohl er zu 90 % aus der Base kommt).

Produces:

- `keph:/docs/generated/code/<NN>-<slug>.md` — Overlay- und Base-Docs, geschrieben über
  `k_playbook_docs_checkout` und `k_playbook_docs_publish` (Phase 5).

## Erster Schritt

include: k-playbook:/commands/_shared/erster-schritt.md

include: k-playbook:/commands/_shared/docs.md

Dieser Skill ist ein Schreiber des Erzeugers `code`: sein Verzeichnis ist
`keph:/docs/generated/code/`, das er sich mit `/k-docs-code` teilt. Seine Dokumente
erkennst du an `generated.by: ks-overlay-repo-analyse`.

Aus dem Briefing brauchst du `context.kephalaion` (`ok`, `address`, `message`) und
`context.project.dir`. Ohne `kephalaion.ok: true` kannst du analysieren, aber nichts
schreiben (siehe die Ablage oben); sag das vor Phase 1.

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
- Was liegt schon in `keph:/docs/generated/code/`? Kephalaion `list` auf
  `docs/generated/code` mit `frontmatter` zeigt Titel und `generated.by` — ohne etwas
  zu holen. Den Bestand holt erst Phase 5.

### Ergebnis

Ein grobes mentales Modell wie:
> „4 Repos, 2 Paare. Overlay-Repos bauen jeweils auf Base-Images auf, die
> ihrerseits als eigene Repos existieren."

---

## Phase 2: Overlay-Analyse

Für jedes Overlay-Repo eine ausführliche Analyse. **Am besten mit
Subagenten parallel**, weil das zeitaufwendig ist.

### Vorgehen mit Subagent

Ein Subagent pro Repo, gestartet parallel. Die Vorlage für den Auftrag steht unter
`k-playbook:/commands/_skills/overlay-repo-analyse/subagent-prompt-overlay.md.template`;
lies sie mit `k_playbook_doc_read`.

Für die Subagenten gelten die Regeln der Docs-Ablage aus dem ersten Schritt; in dieser Phase
schreiben sie gar nichts, auch nicht ins Arbeitsverzeichnis. Wichtige Instruktionen an den
Subagenten:

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

Zwischenstände, die noch mit „⚠️ noch nicht mit Base verglichen" markiert sind.
Sie bleiben in der Sitzung; geschrieben wird erst in Phase 5.

---

## Phase 3: Base beschaffen

Zwei Wege. Idealerweise beide.

### Weg A: Git-Repo clonen (bevorzugt)

```bash
mkdir -p _bases   # falls Snapshots existieren
if command -v git >/dev/null 2>&1; then
  git clone https://github.com/<org>/<base-repo>.git <base-repo>

  # Auf den Tag/Commit auschecken, der im Overlay gepinnt ist
  cd <base-repo>
  git checkout <sha aus dem <timestamp>-<sha>-Tag>
else
  echo "git fehlt — Weg B nehmen"
fi
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
if command -v git >/dev/null 2>&1; then
  cd <base-repo>
  git checkout -b _tmp_diff_check   # nicht auf main arbeiten

  cp -r ../<overlay-repo>/app/* app/

  git status                # welche Dateien anders
  git diff --stat           # Zeilen-Statistik pro Datei
  git diff                  # voller Diff

  # Zurück
  git checkout main && git branch -D _tmp_diff_check
else
  echo "git fehlt — Rezept 1 bis 4 nehmen"
fi
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

Jetzt die Erkenntnisse in strukturierte Docs überführen — auf demselben Weg wie
`/k-docs-code`: checkout, im Arbeitsverzeichnis schreiben, publish.

### Wohin die Docs gehen

Alle Docs dieses Skills — Overlay-Docs **und** Base-Docs — gehen nach
`keph:/docs/generated/code/`. Das Verzeichnis trägt die Herkunft „aus Code
abgeleitet", nicht ein einzelnes Werkzeug: `/k-docs-code` schreibt ebenfalls
dorthin. Dass eine Base-Doc fremden Code beschreibt, der gar nicht im Projekt
liegt, ändert an der Herkunft nichts.

### checkout

Rufe `k_playbook_docs_checkout` mit `projectDir` = `context.project.dir` und
`erzeuger: code` (ersatzweise `k-playbook docs checkout code --json`). `WORK_DIR` ist
`dir` aus der Antwort.

- `state: existing` — ein Arbeitsverzeichnis liegt schon da. Nenne `existing.since`,
  `existing.ageMinutes` und `existing.files` und frage: weitermachen oder verwerfen
  (`discard: true`)? Ohne Antwort geht es nicht weiter.
- `kephalaion_unavailable` — nichts wird geschrieben (siehe die Ablage oben). Die
  Ergebnisse der Analyse in der Antwort zusammenfassen, keine Dateien auf der Platte.

### Bestand lesen

Lies alle Dokumente in `WORK_DIR`, bevor du schreibst: `generated.by`, `hints` und
`questions` aus dem Frontmatter, den Text als Anregung. `hints` befolgst du.

- **Zuständig** ist dieser Lauf nur für Dokumente mit
  `generated.by: ks-overlay-repo-analyse`, die er in dieser Analyse behandelt.
- Ein Dokument mit `generated.by: k-docs-code` — und jedes andere, das dieser Lauf nicht
  behandelt — bleibt im Arbeitsverzeichnis **unverändert liegen**, samt `hints` und
  `questions`. Nicht löschen, nicht umbenennen, nicht umschreiben: `publish` deutet eine
  fehlende Datei als Löschung.
- Neue Dateien bekommen die nächsten **freien Nummern im Arbeitsverzeichnis**:
  `<NN>-<slug>.md`, Einer-Schritte mit bewussten Lücken (`00`, `01`, …, `90`), nicht blind
  bei `00` beginnen. Ein umgeschriebenes Dokument behält seinen Namen.

Zeige vor dem Schreiben die Liste: neu, umschreiben, entfällt (nur eigene), bleibt —
dazu vorgeschlagene neue `hints` — und warte auf Bestätigung.

### Frontmatter und Inhalt

Jede Datei trägt den Kopf aus dem Abschnitt „Der Kopf“ der Docs-Ablage im ersten Schritt —
`title`, `description`, `generated`, `hints`, `questions`, bei Bedarf `sources`; mehr nicht.
Wie dieser Skill ihn belegt, zeigt die Vorlage
`k-playbook:/commands/_skills/overlay-repo-analyse/overlay-doc.md.template` (lies sie mit
`k_playbook_doc_read`): `generated: { by: ks-overlay-repo-analyse, at: <GENERATED_AT> }`,
`GENERATED_AT` wie in der Docs-Ablage. Den Scope der Docs-Ablage wendet dieser Skill nicht
an; er analysiert die Repos aus Phase 1.

Verwandte Dokumente im selben Verzeichnis nennst du mit ihrem Dateinamen.

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

### publish

Rufe `k_playbook_docs_publish` mit `projectDir` = `context.project.dir` und
`erzeuger: code` (ersatzweise `k-playbook docs publish code --json`); `allow_hints: true`
nur nach bestätigten `hints`. Melde je Dokument `created`, `written`, `deleted`,
`unchanged`, `conflict` oder `failed`.

- `conflict` — seit dem checkout in Kephalaion geändert. Vorlegen, nicht still
  nachziehen; der Nutzer entscheidet, ob die eigene Änderung verworfen oder nach einem
  neuen checkout (`discard: true`) übertragen wird.
- `hints_changed` oder `invalid_input` — nichts geschrieben; vorlegen bzw. beheben, dann
  erneut.
- `failed` mit `outcome_unknown` — nicht wiederholen, mit Kephalaion `read` nachsehen.

Die `README.md` des Verzeichnisses schreibt `publish`; einen Index schreibt dieser Skill
nicht — er entsteht beim Indizieren.

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
  if command -v git >/dev/null 2>&1; then
    git checkout <sha>
  fi
  ```
- Danach nochmal diffen

### Overlay-Repo hat keine Tests

Häufig bei Overlays. Kann man tun:

- Nur die Deltas testen (nicht die ganze Base-Funktionalität)
- Bei Python: Base-Module per `sys.modules`-Stub mocken, echte Overlay-
  Module testen

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
- [ ] Dokumente anderer Schreiber (`generated.by: k-docs-code`) unverändert gelassen
- [ ] `_bases/`-Verzeichnis ggf. in `.gitignore` (falls Workspace unter Git)
- [ ] Docs mit `publish` nach `keph:/docs/generated/code/` veröffentlicht, nichts offen

---

## Verwandte Commands

- `/k-docs-code` — der zweite Schreiber in `keph:/docs/generated/code/`: Code-Doku des
  Projekts selbst, über denselben Weg checkout → publish.
