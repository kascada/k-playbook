---
description: List open GitHub pull requests for the configured repo target or load a specific PR directly, then present a compact first-pass overview. Phase 1 only: selection plus overview, no deep review yet.
argument-hint: [pr-number|#pr-number|github-pr-url]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, TodoWrite]
---

# k-pr-review

Liste offene GitHub-Pull-Requests fuer das passende Repo und lade einen PR fuer eine erste kompakte Einordnung.

Dieser Command ist bewusst **nur Phase 1**:

- Ohne Argument: offene PRs laden und dem User zur Auswahl zeigen.
- Mit Argument: den angegebenen PR direkt laden.
- Danach den PR knapp vorstellen: Repo, Titel, URL, Autor, Branches, Status, Diff-Umfang, Dateien und Checks/Review-Signale.
- **Keine** tiefe Code-Review, **keine** Merge-Empfehlung, **keine** Remediation und **keine** Schreibzugriffe.

## Invocation

- `/k-pr-review`
- `/k-pr-review 443`
- `/k-pr-review #443`
- `/k-pr-review https://github.com/<owner>/<repo>/pull/443`

## Schritt 1 - Repo-Ziel aufloesen

Read and apply `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.

Danach bestimme das GitHub-Repo fuer den PR-Check in dieser Reihenfolge:

1. Wenn `<TARGET_DIR>/K-PLAYBOOK.yaml` existiert und `remediation.target` gesetzt ist:
   - resolve `remediation.target` relativ zu `TARGET_DIR`
   - verwende diesen Pfad als `PR_TARGET_DIR`
   - das ist der bevorzugte Fall fuer Wrapper-Repos, bei denen das eigentliche Git-Repo nicht das Workspace-Root ist
2. Sonst, wenn `TARGET_DIR` selbst ein Git-Repo ist: verwende `TARGET_DIR` als `PR_TARGET_DIR`
3. Sonst abbrechen mit einer klaren Meldung:
    - entweder im echten Git-Repo starten
   - oder `K-PLAYBOOK.yaml` mit `remediation.target` sauber konfigurieren

Vor dieser Zielwahl gilt zusaetzlich:

- Wenn `<TARGET_DIR>/K-PLAYBOOK.yaml` fehlt: sofort abbrechen und den User auf `/k-gui` verweisen.

Validierung:

- `PR_TARGET_DIR` muss existieren.
- `PR_TARGET_DIR` muss ein Git-Repo sein.
- `gh` muss verfuegbar sein.

Ermittle dann das GitHub-Repo:

```bash
gh repo view --json nameWithOwner,url,defaultBranchRef
```

Fuehre den Befehl in `PR_TARGET_DIR` aus.

Wenn das fehlschlaegt:

- sauber abbrechen und sagen, dass das Repo nicht per `gh` aufloesbar ist oder GitHub-Auth fehlt
- keine heuristische Repo-Raterei aus beliebigen Remotes

Merke:

- `GH_REPO`
- `GH_REPO_URL`
- `GH_DEFAULT_BRANCH`
- `PR_TARGET_DISPLAY`

## Schritt 2 - PR bestimmen

### Fall A - `$ARGUMENTS` ist leer

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

### Fall B - `$ARGUMENTS` ist gesetzt

Akzeptiere nur diese Formen:

- `443`
- `#443`
- `https://github.com/<owner>/<repo>/pull/443`

Normalisierung:

- `#443` -> `443`
- URL unveraendert lassen

Wenn das Argument keine dieser Formen hat:

- mit kurzer Fehlermeldung stoppen
- die gueltigen Formate zeigen

Merke den normalisierten Wert als `PR_SELECTOR`.

## Schritt 3 - PR laden

Wenn `PR_SELECTOR` numerisch ist, lade den PR gegen das bereits aufgeloeste Repo:

```bash
gh pr view <PR_SELECTOR> --repo <GH_REPO> --json number,title,url,state,isDraft,author,createdAt,updatedAt,baseRefName,headRefName,mergeable,reviewDecision,additions,deletions,changedFiles,labels,body
```

Wenn `PR_SELECTOR` eine URL ist, darf direkt gegen diese URL geladen werden:

```bash
gh pr view <PR_SELECTOR> --json number,title,url,state,isDraft,author,createdAt,updatedAt,baseRefName,headRefName,mergeable,reviewDecision,additions,deletions,changedFiles,labels,body
```

Lade zusaetzlich:

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
- Wenn `gh pr view` fehlschlaegt: abbrechen und die konkrete `PR_SELECTOR`-Form nennen, die nicht geladen werden konnte.
- Nicht den kompletten Diff ausgeben.
- Nicht automatisch eine Bewertung oder Merge-Freigabe ableiten.

## Schritt 4 - Kompakten PR-Ueberblick ausgeben

Stelle den geladenen PR kompakt vor. Ziel: der User soll schnell verstehen, **welcher** PR das ist und **welche Groesse/Signale** er hat.

Pflichtfelder:

```text
PR-Ueberblick
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
  - wenn mehr als 15 Dateien geaendert wurden: `... plus <N> weitere`

Keine weiteren Interpretationen in dieser Phase, ausser einer knappen Einordnung wie:

- `Sieht nach kleinem lockfile-only Update aus.`
- `Wirkt wie ein breiterer Multi-File-PR; Detailpruefung waere der naechste Schritt.`

Diese Einordnung muss rein deskriptiv bleiben und darf noch keine Merge-Empfehlung enthalten.

## Schritt 5 - Abschluss

Am Ende kurz sagen, dass Phase 1 abgeschlossen ist und der PR jetzt fuer den naechsten Schritt bereit ist.

Beispiel:

```text
Phase 1 abgeschlossen. Wenn du willst, beurteile ich als Naechstes Risiko, Checks, Diff-Scope und Merge-Eignung dieses PRs.
```

## Fehlerfaelle

- `K-PLAYBOOK.yaml` fehlt -> sauber abbrechen und `/k-gui` nennen
- `Remediation.target` ist gesetzt, aber der Pfad fehlt oder ist kein Git-Repo -> sauber abbrechen
- `gh` fehlt oder ist nicht authentifiziert -> sauber abbrechen und das Problem klar benennen
- offene PR-Liste ist leer -> melden und stoppen
- ungueltiges Argument -> gueltige Formate nennen und stoppen
- PR nicht gefunden -> Repo + Selector nennen und stoppen
