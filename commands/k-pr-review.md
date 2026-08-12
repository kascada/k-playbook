---
description: List open GitHub pull requests for the configured repo target or load a specific PR directly, then present a compact overview, a PR assessment, and an optional follow-up action: approve, merge, or create a local validation branch.
argument-hint: [pr-number|#pr-number|github-pr-url] [quick|standard|deep]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, TodoWrite]
---

# k-pr-review

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Liste offene GitHub-Pull-Requests für das passende Repo, lade einen PR und führe ihn durch drei Phasen mit einer optionalen Folgeaktion: approven, mergen oder einen lokalen Validierungs-Branch anlegen.

- **Phase 1:** Auswahl plus kompakter PR-Überblick.
- **Phase 2:** read-only PR-Bewertung in `quick`, `standard` oder `deep`.
- **Phase 3:** Empfehlung plus optionaler Folgeaktion.

Der Command bleibt bewusst konservativ:

- keine Produktcode-Änderungen
- keine Review-Artefakte unter `k-playbook-local/results/`
- keine Remediation
- keine Protokollierung angenommener PRs in diesem Schritt
- kein automatischer Merge ohne explizite User-Anfrage

Zulässige schreibende Folgeaktionen nur nach expliziter User-Entscheidung:

- PR auf GitHub approven
- PR auf GitHub mergen
- lokalen Validierungs-Branch für weitergehende Tests anlegen

## Invocation

- `/k-pr-review`
- `/k-pr-review 443`
- `/k-pr-review #443`
- `/k-pr-review https://github.com/<owner>/<repo>/pull/443`
- `/k-pr-review quick`
- `/k-pr-review 443 quick`
- `/k-pr-review #443 standard`
- `/k-pr-review https://github.com/<owner>/<repo>/pull/443 deep`

## Schritt 0 - Argumente normalisieren

Der Command akzeptiert bis zu zwei optionale Argumente:

1. PR-Selektor
2. Bewertungsmodus

Gültige PR-Selektoren:

- `443`
- `#443`
- `https://github.com/<owner>/<repo>/pull/443`

Gültige Bewertungsmodi:

- `quick`
- `standard`
- `deep`

Zulässige Aufrufe:

- kein Argument
- nur PR-Selektor
- nur Bewertungsmodus
- PR-Selektor gefolgt von Bewertungsmodus

Wenn mehr als zwei Argumente übergeben werden: mit kurzer Fehlermeldung stoppen und die gültigen Formen zeigen.

Normalisierung:

- `#443` -> `443`
- GitHub-PR-URL unverändert lassen
- Modus immer in Kleinschreibung merken

Wenn nur ein Argument übergeben wird:

- wenn es ein PR-Selektor ist: `PR_SELECTOR` setzen, `ASSESSMENT_MODE` offen lassen
- wenn es ein Bewertungsmodus ist: `ASSESSMENT_MODE` setzen, `PR_SELECTOR` offen lassen
- sonst mit kurzer Fehlermeldung stoppen

Wenn zwei Argumente übergeben werden:

- das erste muss ein gültiger PR-Selektor sein
- das zweite muss ein gültiger Bewertungsmodus sein
- sonst mit kurzer Fehlermeldung stoppen

## Schritt 1 - Repo-Ziel auflösen

Aus der Context-Ausgabe:

- `PR_TARGET_DIR` = `project.repoRoot`. Das deckt auch Wrapper-Repos ab, bei denen das eigentliche Git-Repo nicht das Hauptverzeichnis ist.
- `DOCS_DIR` = `<local.dir>/docs` — Phase 2 braucht es für die Docs-Sync-Pflicht.
- `EFFECTIVE_RULES` = `catalogs.rules`, inklusive Herkunft je Regel.

Wenn `remediation.target` gesetzt und nicht `.` ist, benennt es den engeren Code-Root
innerhalb des Repos; nutze ihn für die Code-Sichtung, aber nicht als `PR_TARGET_DIR`.

Wenn `project.vcs` nicht `git` ist: abbrechen, ein PR-Review braucht ein Git-Repo.

Validierung:

- `PR_TARGET_DIR` muss existieren.
- `PR_TARGET_DIR` muss ein Git-Repo sein.
- `gh` muss nutzbar sein. Das steht in `gh` aus der Context-Ausgabe und wird nicht selbst
  ermittelt:
  - `gh.status: disabled` — abbrechen; dieses Projekt hat sich gegen gh entschieden, ein
    PR-Review ist damit nicht vorgesehen.
  - `gh.status: unknown` — abbrechen und `/k-gui` nennen; dort fällt die Entscheidung.
  - `gh.ready: false` — abbrechen und benennen, was fehlt: Installation von `gh` oder
    `gh auth login --hostname github.com`. Beides gehört ins Terminal des Users.
- Merke `GH_ACCOUNT` = `gh.account`. Schritt 9 nennt ihn vor jeder Schreibaktion.

Ermittle dann das GitHub-Repo:

```bash
gh repo view --json nameWithOwner,url,defaultBranchRef
```

Führe den Befehl in `PR_TARGET_DIR` aus.

Wenn das fehlschlägt:

- sauber abbrechen und sagen, dass das Repo nicht per `gh` auflösbar ist oder GitHub-Auth fehlt
- keine heuristische Repo-Raterei aus beliebigen Remotes

Merke:

- `GH_REPO`
- `GH_REPO_URL`
- `GH_DEFAULT_BRANCH`
- `PR_TARGET_DISPLAY`
- `DOCS_DIR`
- `EFFECTIVE_RULES`

## Schritt 2 - PR bestimmen

### Fall A - kein Argument übergeben

Lade die offenen PRs:

```bash
gh pr list --repo <GH_REPO> --state open --limit 30 --json number,title,author,baseRefName,headRefName,isDraft,updatedAt,url
```

Wenn keine offenen PRs existieren:

- melde ehrlich: `Keine offenen Pull Requests gefunden.`
- stoppe

Wenn offene PRs existieren:

- zeige eine kompakte Auswahl, sortiert nach `updatedAt` absteigend
- Format:

```text
Offene Pull Requests
────────────────────
Repo: <GH_REPO>

[1] #443 chore: update shell-quote dependency
    draft: nein   author: dependabot[bot]   updated: 2026-07-31
    base: main    head: dependabot/npm_and_yarn/theme/static_src/shell-quote-1.8.3

[2] #441 ...
    ...

Welchen PR laden?
Antworte mit `443`, `#443` oder einer GitHub-PR-URL.
```

- danach auf die Auswahl des Users warten

### Fall B - Argument übergeben

Wenn `PR_SELECTOR` aus Schritt 0 bereits gesetzt ist, diesen Wert verwenden.

Wenn `PR_SELECTOR` nach Schritt 0 noch leer ist, aber der User gerade einen PR aus der Liste auswählt:

- dieselben drei PR-Formen akzeptieren
- `#443` zu `443` normalisieren
- URL unverändert lassen
- sonst mit kurzer Fehlermeldung stoppen

Merke den normalisierten Wert als `PR_SELECTOR`.

## Schritt 3 - PR laden

Wenn `PR_SELECTOR` numerisch ist, lade den PR gegen das bereits aufgelöste Repo:

```bash
gh pr view <PR_SELECTOR> --repo <GH_REPO> --json number,title,url,state,isDraft,author,createdAt,updatedAt,baseRefName,headRefName,mergeable,reviewDecision,additions,deletions,changedFiles,labels,body
```

Wenn `PR_SELECTOR` eine URL ist, darf direkt gegen diese URL geladen werden:

```bash
gh pr view <PR_SELECTOR> --json number,title,url,state,isDraft,author,createdAt,updatedAt,baseRefName,headRefName,mergeable,reviewDecision,additions,deletions,changedFiles,labels,body
```

Lade zusätzlich:

- bei numerischem `PR_SELECTOR`:

```bash
gh pr diff <PR_SELECTOR> --repo <GH_REPO> --name-only
gh pr checks <PR_SELECTOR> --repo <GH_REPO>
```

- bei URL-`PR_SELECTOR`:

```bash
gh pr diff <PR_SELECTOR> --name-only
gh pr checks <PR_SELECTOR>
```

Regeln:

- Wenn `gh pr checks` keine Checks findet oder non-zero endet, das als `keine oder keine sauber abrufbaren Checks` behandeln, nicht als harten Command-Fehler.
- Wenn `gh pr view` fehlschlägt: abbrechen und die konkrete `PR_SELECTOR`-Form nennen, die nicht geladen werden konnte.
- Nicht den kompletten Diff ausgeben.
- Noch keine Merge-Freigabe ableiten.

## Schritt 4 - Kompakten PR-Überblick ausgeben

Stelle den geladenen PR kompakt vor. Ziel: der User soll schnell verstehen, **welcher** PR das ist und **welche Größe/Signale** er hat.

Pflichtfelder:

```text
PR-Überblick
─────────────
Repo:        <GH_REPO>
Pfad:        <PR_TARGET_DISPLAY>
PR:          #<number> <title>
URL:         <url>
Status:      <OPEN/CLOSED/MERGED>, draft <ja/nein>
Autor:       <author>
Branches:    <head> -> <base>
Zeit:        erstellt <createdAt>, aktualisiert <updatedAt>
Review:      <reviewDecision oder "—">
Mergebar:    <mergeable oder "unknown">
Diff:        <changedFiles> Dateien, +<additions> / -<deletions>
Labels:      <comma list oder "—">
Checks:      <kurze Zusammenfassung aus `gh pr checks` oder "—">
```

Danach:

- `Kurzbeschreibung:` erste sinnvolle Zeile oder erster Absatz aus dem PR-Body; wenn leer: `—`
- `Dateien:`
  - bis zu 15 Dateipfade einzeln anzeigen
  - wenn mehr als 15 Dateien geändert wurden: `... plus <N> weitere`

Keine weiteren Interpretationen in dieser Phase, außer einer knappen Einordnung wie:

- `Sieht nach kleinem lockfile-only Update aus.`
- `Wirkt wie ein breiterer Multi-File-PR; Detailprüfung wäre der nächste Schritt.`

Diese Einordnung muss rein deskriptiv bleiben und darf noch keine Merge-Empfehlung enthalten.

## Schritt 5 - Bewertungsmodus bestimmen

Wenn `ASSESSMENT_MODE` aus Schritt 0 bereits gesetzt ist: direkt verwenden.

Wenn `ASSESSMENT_MODE` noch leer ist, frage den User nach genau einem Modus:

```text
Welchen Bewertungsmodus soll ich für diesen PR verwenden?

- quick: GitHub-Signale, Diff-Scope und Enforcement-Einschätzung
- standard: quick plus `k-check --mode changed`
- deep: standard plus gezielte lokale Validierung je nach PR-Scope
```

Akzeptiere nur `quick`, `standard` oder `deep`.

## Schritt 6 - Read-only PR-Bewertung

Ziel: den PR anhand vorhandener k-playbook-Regeln und Checks knapp bewerten, ohne Review-Artefakte zu schreiben.

### 6.1 Bewertungsquellen

Lade für die Bewertung:

- die effektive Regelmenge `EFFECTIVE_RULES` aus `catalogs.rules`
- projektlokale Docs aus `DOCS_DIR`, soweit für Docs-Sync sichtbar relevant

Nutze diese Quellen als Constraints, nicht als Anlass für ein separates `/k-review`.

Wichtig:

- `catalogs.reviews` ist in diesem Command nur Referenz für mögliche Folge-Schritte, nicht der Default-Executor.
- keine Dateien unter `<local.dir>/results/` oder `<local.dir>/tasks/` schreiben.
- nur read-only Kommandos und Analyse.

### 6.2 PR-Scope klassifizieren

Ordne die geänderten Dateien knapp ein, z. B.:

- `docs-only`
- `dependency-only`
- `lockfile-only`
- `django-runtime`
- `django-models-migrations`
- `auth-or-user-scope`
- `logging-privacy`
- `frontend-copy`
- `multi-area`

Nutze diese Klassifikation für die spätere Relevanzbewertung der Regeln und Checks.

### 6.3 Quick-Modus

`quick` führt nur leichte read-only Bewertung aus:

- GitHub-Checks und Review-Signale aus Schritt 3 auswerten
- Diff-Scope und Dateitypen einordnen
- relevante Enforcement-Regeln benennen und gegen den Scope prüfen
- insbesondere immer die globale Docs-Sync-Regel prüfen

Für Enforcement gilt:

- Nur Regeln aus `EFFECTIVE_RULES` heranziehen. Eine abgeschaltete Regel wird nicht geprüft, auch wenn sie mitgeliefert wird.
- `docs-sync.md` immer prüfen, wenn Code-Dateien geändert wurden und die Regel in `EFFECTIVE_RULES` enthalten ist
- Django-Validierung nur dann als relevant markieren, wenn PR-Dateien Settings, Middleware, URLs, Models, Migrations, Storage, Redis, Celery, Helm oder Runtime-Startpfade berühren
- User-Data-Isolation, Logging-Privacy und i18n nur dann als relevant markieren, wenn die geänderten Dateien plausibel in diesen Bereich fallen

Es werden in `quick` keine zusätzlichen lokalen Validierungsbefehle gestartet.

### 6.4 Standard-Modus

`standard` enthält alles aus `quick` und lässt zusätzlich den globalen Check-Runner auf dem PR-Scope laufen.

Vorgehen:

1. Erzeuge eine temporäre Datei außerhalb des Projekts, z. B. via `mktemp`, mit der newline-separierten PR-Dateiliste aus Schritt 3.
2. Führe aus dem Projektkontext aus:

```bash
<playbook.dir>/bin/k-check \
  --config-root <project.dir> \
  --target-root <PR_TARGET_DIR> \
  --mode changed \
  --files-from <temp-file>
```

Regeln:

- Keine `--output`- oder `--metadata-output`-Dateien verwenden; dieser Command bleibt read-only bezüglich Projektartefakten.
- `exit 1` von `k-check` als fachliche Findings behandeln, nicht als technischen Command-Fehler.
- `exit 2` oder fehlende Runner-Voraussetzungen als `k-check technisch nicht sauber ausführbar` berichten.
- Die temporäre Datei am Ende entfernen.

### 6.5 Deep-Modus

`deep` enthält alles aus `standard` und führt zusätzlich die kleinste sinnvolle lokale Validierung passend zum Scope aus.

Nutze nur read-only Validierung und bleibe eng am PR-Scope. Beispiele:

- Wenn Django-Runtime-Dateien geändert wurden und `manage.py` vorhanden ist: `python manage.py check`
- Wenn models-/migrationsrelevante Dateien geändert wurden: `python manage.py makemigrations --check --dry-run`
- Wenn der Scope offensichtliche fokussierte Tests hergibt: den schmalsten sinnvollen Testlauf nennen und nur in `deep` ausführen
- Bei `docs-only`, `dependency-only` oder `lockfile-only` keine künstlichen Django- oder Testläufe erzwingen

Wenn eine sinnvolle Deep-Validierung nicht sicher bestimmbar ist, nicht raten: kurz melden, dass `deep` keine zusätzliche sichere lokale Validierung ableiten konnte.

### 6.6 Bewertungslogik

Bewerte den PR knapp und deskriptiv anhand dieser Signale:

- GitHub-Checks
- PR-Scope
- `k-check`-Ergebnis, falls gelaufen
- Enforcement-Relevanz und Enforcement-Offenpunkte
- tiefe lokale Validierung, falls gelaufen

Verwende eine einfache Risikoeinschätzung:

- `niedrig`
- `mittel`
- `hoch`
- `unklar`

Diese Risikoeinschätzung ist keine Merge-Freigabe.

Leite zusätzlich eine Handlungsempfehlung ab:

- `direkt annehmen`
- `branch erstellen und weiter testen`

Faustregeln für `direkt annehmen`:

- Risiko `niedrig`
- keine `k-check`-Fails oder technischen `error`
- keine offenen Enforcement-Pflichten
- kein Hinweis auf sensible oder breitflächige Runtime-/Auth-/Ownership-Änderungen ohne ausreichende lokale Validierung

Faustregeln für `branch erstellen und weiter testen`:

- Risiko `mittel`, `hoch` oder `unklar`
- `k-check` meldet `fail` oder `error`
- Enforcement offen oder unklar
- Branch-Checks fehlen und der Scope berührt sensible Bereiche wie Auth, Runtime, Models/Migrations, Ownership oder Logging/Privacy
- es gibt eine sinnvolle weitergehende lokale Validierung, die über den aktuellen Modus hinausgeht

## Schritt 7 - Bewertung ausgeben

Stelle die Bewertung kompakt dar:

```text
PR-Bewertung
────────────
Modus:        <quick|standard|deep>
Scope:        <klassifikation>
Risiko:       <niedrig|mittel|hoch|unklar>
GitHub:       <Kurzfassung Checks + Review-Signale>
k-check:      <nicht gelaufen | Kurzfassung | technisch nicht sauber ausführbar>
Enforcement:  <relevante Regeln und Ergebnis in einem Satz>
Validierung:  <nicht gelaufen | Liste der Deep-Checks mit Kurzresultat>
Docs-Sync:    angepasst | nicht nötig (<Grund>) | fehlt | unklar
```

Danach knapp:

- `Auffällig:` wichtigste 1-3 Signale oder offene Punkte
- `Empfehlung:` `direkt annehmen` oder `branch erstellen und weiter testen` mit kurzem Grund

Noch nicht enthalten in diesem Schritt:

- keine Protokollierung in `k-playbook-local/results/` oder anderswo
- kein automatischer Handoff nach `/k-review` oder `/k-remediation`

## Schritt 8 - Folgeaktion wählen

Nach der Bewertung frage den User nach genau einer Folgeaktion:

```text
Wie soll ich mit diesem PR weiter verfahren?

- direkt annehmen
- branch erstellen und weiter testen
- nichts weiter
```

Wenn die Empfehlung `direkt annehmen` ist, nenne diese Option zuerst.
Wenn die Empfehlung `branch erstellen und weiter testen` ist, nenne diese Option zuerst.

## Schritt 9 - Folgeaktion ausführen

Approve, Merge und Kommentar sind nach außen sichtbar und laufen unter dem Account, der
auf diesem Rechner gerade aktiv ist — der gilt maschinenweit und kann seit dem letzten
Mal in einem anderen Projekt umgeschaltet worden sein. Nenne deshalb vor der ersten
Schreibaktion `GH_ACCOUNT` in einer Zeile, etwa:

```
Ausgeführt wird als GitHub-Account <GH_ACCOUNT>.
```

Ist `GH_ACCOUNT` leer, läuft die Anmeldung über `GH_TOKEN`/`GITHUB_TOKEN`; sag das
genauso, statt einen Namen zu erfinden.

### 9.1 Direkt annehmen

Wenn der User `direkt annehmen` wählt:

1. Erzeuge für den Approval-Text eine temporäre Datei außerhalb des Projekts, z. B. via `mktemp`, und schreibe den Text dort hinein.

   Regel:

   - Für `gh pr review`, `gh pr comment` oder ähnliche PR-Kommentare niemals mehrzeilige Texte inline per `-b "...\n..."` oder `--body "...\n..."` bauen.
   - Stattdessen immer eine temporäre Datei und `--body-file <temp-file>` verwenden, damit Shell-Interpolation, Backticks und Newlines nicht zerbrechen.

2. Führe ein GitHub-Approval aus:

   - numerischer Selektor:

   ```bash
   gh pr review <PR_SELECTOR> --repo <GH_REPO> --approve --body-file <temp-file>
   ```

   - URL-Selektor:

   ```bash
   gh pr review <PR_SELECTOR> --approve --body-file <temp-file>
   ```

3. Approval-Text kurz und sachlich halten, z. B. Scope, Modus und wichtigste Signale.
4. Die temporäre Datei am Ende entfernen.
5. Danach kurz den User fragen, ob der PR nur approvt oder direkt gemerged werden soll.

   Optionen:

   - `nur approven`
   - `jetzt mergen`

6. Wenn der User nur approven will: keinen Merge per CLI ausführen und im Abschluss sagen, dass der Merge online auf GitHub erfolgen kann.
7. Wenn der User `jetzt mergen` will: weiter mit Schritt 9.3.

Wenn der Approval-Call fehlschlägt:

- den Grund kurz und klar berichten
- insbesondere bei Self-Approval-Fällen knapp sagen, dass GitHub den eigenen PR nicht approven lässt
- keinen automatischen Fallback-Merge versuchen
- stattdessen einen kurzen Merge-Hinweis als kopierbaren Markdown-Block ausgeben, damit der User ihn bei Bedarf manuell auf GitHub einfügen kann

Beispiel für den auszugebenden Merge-Hinweis:

```markdown
k-playbook PR-Einschätzung (`<mode>`):

- Scope: `<scope>`
- Risiko: `<risk>`
- `k-check`: `<summary>`
- Enforcement: `<summary>`
- Validierung: `<summary>`

Hinweis: CLI-Approval war nicht möglich (`<kurzer Grund>`). Wenn Berechtigungen und Branch-Regeln es zulassen, kann der Merge bewusst online auf GitHub erfolgen.
```

### 9.2 Branch erstellen und weiter testen

Wenn der User `branch erstellen und weiter testen` wählt:

1. Vor Branch-Wechsel den Worktree in `PR_TARGET_DIR` prüfen:

   ```bash
   git status --short
   ```

2. Wenn der Worktree nicht sauber ist: stoppen und den User bitten zu entscheiden. Nicht automatisch stashen, resetten oder fremde Änderungen bewegen.

3. Einen lokalen Validierungs-Branch vom PR-Head anlegen und auschecken.

   Branch-Name:

   ```text
   pr-review/<PR-number>-<kurzer-slug>
   ```

   Beispiel:

   ```text
   pr-review/441-python-jose
   ```

4. Branch anlegen via GitHub-PR-Checkout:

   - numerischer Selektor:

   ```bash
   gh pr checkout <PR_SELECTOR> --repo <GH_REPO> --branch <LOCAL_VALIDATION_BRANCH>
   ```

   - URL-Selektor:

   ```bash
   gh pr checkout <PR_SELECTOR> --branch <LOCAL_VALIDATION_BRANCH>
   ```

5. Danach den kleinsten sinnvollen erweiterten Testlauf passend zum Scope ausführen.

Beispiele:

- `dependency-only` für authnahe Python-Library: `pip install -r requirements.txt`, `python manage.py check`, `python manage.py makemigrations --check --dry-run`, fokussierte Auth-/Refresh-/JWT-nahe Tests
- `django-runtime`: `python manage.py check` plus fokussierte Runtime-/Endpoint-Tests
- `django-models-migrations`: `python manage.py makemigrations --check --dry-run` plus betroffene Model-/View-Tests
- `docs-only`: kein künstlicher Testbranch nötig; wenn der User ihn trotzdem will, nur Branch anlegen und das so benennen

6. Keine Produktcode-Änderungen im Test-Branch machen, solange der User nicht explizit in einen Implementierungs-/Fix-Flow wechselt.
7. Den Testbranch nicht automatisch pushen.

8. Nach erfolgreichem Testlauf klar sagen:

   - der lokale Validierungs-Branch ist nur ein Prüf-Branch
   - merge-relevant bleibt der ursprüngliche PR `<head> -> <base>`

9. Danach erneut nach der Abschlussentscheidung fragen:

```text
Der erweiterte Testlauf ist durch. Wie soll ich mit dem ursprünglichen PR weiter verfahren?

- PR approven
- PR mergen
- nichts weiter
```

Wenn der User `PR approven` oder `PR mergen` wählt, weiter mit Schritt 9.3.
Wenn der User `nichts weiter` wählt, weiter mit Schritt 9.4 für Aufräumen und Abschluss.

### 9.3 PR mergen oder approven

Dieser Schritt arbeitet immer auf dem ursprünglichen PR, nie auf dem lokalen Validierungs-Branch.

#### 9.3.a Approven

- Wenn noch kein Approval versucht wurde, führe denselben Approval-Flow wie in 9.1 aus.
- Wenn Approval wegen Self-Approval nicht möglich ist, klar sagen, dass GitHub den eigenen PR nicht approven lässt.
- Wenn der User danach trotzdem `PR mergen` verlangt und der Repo-/Branch-Schutz es erlaubt, darf der Command den Merge auf ausdrückliche Anweisung trotzdem ausführen.

#### 9.3.b Mergen

Merge nur wenn der User es ausdrücklich verlangt.

Vor dem Merge:

- PR noch einmal kurz prüfen: `state`, `mergeable`, `baseRefName`, `headRefName`
- Wenn `mergeable` klar negativ ist oder der PR nicht mehr offen ist: sauber stoppen

Merge-Befehl:

- numerischer Selektor:

```bash
gh pr merge <PR_SELECTOR> --repo <GH_REPO> --merge
```

- URL-Selektor:

```bash
gh pr merge <PR_SELECTOR> --merge
```

Regeln:

- keinen `--squash`, `--rebase`, `--admin` oder Force-Workaround raten oder automatisch nutzen
- nur den Standard-Merge ausführen, sofern der User keinen anderen Merge-Typ explizit verlangt
- wenn GitHub/Branch-Protection den Merge blockiert, den Grund klar berichten

### 9.4 Lokalen Validierungs-Branch aufräumen

Wenn in 9.2 ein lokaler Validierungs-Branch angelegt wurde:

1. Vor dem Löschen prüfen:

```bash
git status --short
```

2. Wenn der Worktree nicht sauber ist: den Branch nicht automatisch löschen; klar berichten und stoppen.

3. Wenn sauber:

   - auf einen sicheren Branch wechseln, bevorzugt den Base-Branch des PR (`<baseRefName>`), sonst den ursprünglichen Start-Branch
   - lokalen Validierungs-Branch löschen

Beispiel:

```bash
git switch <baseRefName>
git pull --ff-only origin <baseRefName>
git branch -D <LOCAL_VALIDATION_BRANCH>
```

Wenn der ursprüngliche Start-Branch der PR-Head-Branch war und der PR gemerged wurde, ist der Base-Branch der bevorzugte Rückkehrpunkt.

### 9.5 Nichts weiter

Wenn der User `nichts weiter` wählt: nur kompakt bestätigen und keine Schreibaktion ausführen.

## Schritt 10 - Abschluss und Sauberkeitsprüfung

Führe am Ende immer eine kurze Repo-Sauberkeitsprüfung aus:

```bash
git status --short --branch
```

Wenn ein PR gemerged wurde und du bereits auf dem Base-Branch stehst, ziehe zusätzlich den aktuellen Stand per Fast-Forward nach.

Am Ende kurz sagen, was ausgeführt wurde:

- nur Überblick plus Bewertung
- oder Approval erfolgt, Merge bitte online auf GitHub
- oder PR gemerged
- oder lokaler Validierungs-Branch angelegt, erweiterter Testlauf ausgeführt und Branch wieder aufgeräumt

Beispiel:

```text
PR bewertet und auf GitHub approvt. Bitte den Merge bewusst online auf GitHub ausführen.
```

Oder:

```text
PR gemerged. Lokaler Validierungs-Branch `pr-review/441-python-jose` wurde anschließend entfernt und der Repo-Zustand ist sauber.
```

## Fehlerfälle

- Kein k-playbook-Projekt (Context-Aufruf schlägt fehl) -> sauber abbrechen und `/k-gui` empfehlen
- `project.repo_root` oder der Legacy-Fallback `remediation.target` ist gesetzt, aber der Pfad fehlt oder ist kein Git-Repo -> sauber abbrechen
- `gh` fehlt oder ist nicht authentifiziert -> sauber abbrechen und das Problem klar benennen
- mehr als zwei Argumente oder ungültige Argument-Kombination -> gültige Formen nennen und stoppen
- offene PR-Liste ist leer -> melden und stoppen
- ungültiger PR-Selektor -> gültige Formate nennen und stoppen
- ungültiger Bewertungsmodus -> `quick`, `standard`, `deep` nennen und stoppen
- PR nicht gefunden -> Repo + Selector nennen und stoppen
- Approval fehlgeschlagen -> klar berichten, kein Merge-Fallback
- PR-Kommentar-/Approval-Body wäre inline shell-anfällig -> immer `--body-file` mit temp Datei verwenden
- lokaler Worktree für Test-Branch nicht sauber -> stoppen und User entscheiden lassen
- Branch-Name existiert bereits oder Checkout fehlgeschlagen -> klar berichten und nicht improvisieren
- Merge von GitHub blockiert -> Grund klar berichten, keinen Workaround erzwingen
- lokaler Validierungs-Branch kann wegen uncleanem Worktree nicht gelöscht werden -> klar berichten
