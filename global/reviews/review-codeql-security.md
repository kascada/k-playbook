---
name: review-codeql-security
title: CodeQL Security Assessment
interval-weeks: 4
scope-hint: CodeQL SARIF-Ergebnisse fuer die in K-PLAYBOOK.MD registrierten Sprachen; keine Remediation, keine Code-Aenderungen
handoff: /k-remediation
result-family: codeql
---

# Review: CodeQL Security Assessment

Erzeuge eine kuratierte Security-Bewertung aus CodeQL-Ergebnissen. Dieses Review ist eine **Scan-Familie** im Security-Review-Prozess: CodeQL/SAST fuer eigene Codefehler. Es ersetzt keine Dependency-CVE-, Secret-, Config- oder IaC-Scans.

Der generische Rahmen wird von `/k-review` orchestriert. Diese Datei beschreibt nur die CodeQL-spezifische Analyse, Preflight-Policy und Report-Struktur.

## Zweck

- CodeQL-Rohmeldungen aus SARIF in belastbare Review-Ergebnisse ueberfuehren.
- CodeQL-Metadaten (`security-severity`, `problem.severity`, `precision`, CWE/CVE-Tags) sichtbar machen.
- Automatische Scanner-Meldungen von manueller Review-Bewertung trennen.
- Priorisierte Findings fuer spaetere Remediation vorbereiten.
- Keine Produktcode-Aenderungen durch dieses Review.

## Begriffe

- **Scan-Familie**: Ein zusammenhaengender Review-Baustein mit eigenem Tooling und eigener Bewertungslogik, z. B. CodeQL, Dependency-CVE, Secret-Scanning, Config/IaC.
- **Rohdaten**: Maschinenlesbare Tool-Ausgabe im Ergebnisverzeichnis des Review-Laufs; bei CodeQL SARIF-Dateien.
- **Assessment**: Kuratierte menschenlesbare Bewertung im Ergebnisverzeichnis des Review-Laufs.
- **Finding-Register**: Vollstaendige, einzeln statusfaehige Liste aller Tool-Findings.
- **Ergebnisverzeichnis**: `k-playbook/reviews/results/codeql/YYYY-MM-DD/` fuer einen CodeQL-Review-Lauf.

## Voraussetzungen

Dieses Review darf nur laufen, wenn das Zielprojekt CodeQL bereits ueber `/k-setup-codeql` registriert hat.

Pfad- und Statusauflosung:

- Lies und verwende `<PLAYBOOK_REPO>/commands/_shared/path-resolution.md`.
- Wenn `K-PLAYBOOK.MD` fehlt: abbrechen und `/k-setup` + `/k-setup-codeql` nennen.
- Wenn kein CodeQL-Managed-Block vorhanden ist: abbrechen und `/k-setup-codeql` nennen.
- Wenn weder `github:` noch `local-database:` `true` oder `planned` ist: abbrechen, weil CodeQL fuer dieses Projekt nicht aktiviert ist.
- Wenn `k-playbook/reviews` fehlt: abbrechen und `/k-setup` nennen; dieses Review braucht ein lokales `reviews`-Ziel fuer den Assessment-Report.
- `checks` ist fuer ausfuehrbare Pruefroutinen reserviert. SARIF- und Report-Ergebnisse gehoeren nicht dauerhaft nach `k-playbook/checks/`.

Zu lesen aus dem CodeQL-Block:

- `target:`
- `github:`
- `workflow:`
- `local-database:`
- `database:`
- `languages:`
- `queries:`

## Preflight

Pruefe kompakt:

```text
/k-review codeql-security — Preflight
────────────────────────────────────
Projekt:           <TARGET_DIR>
CodeQL Target:     <target oder .>
K-PLAYBOOK.MD:     gefunden
CodeQL Block:      gefunden
GitHub CodeQL:     true | false | planned
Workflow:          <path> | - | fehlt
Lokale DB:         true | false | planned
Datenbankpfad:     <path> | - | fehlt
CodeQL CLI:        ok (<version>) | fehlt
Sprachen:          <languages> | unklar
Queries:           <queries> | security-extended
Reviews:           <PROJECT_REVIEWS_DIR>
Result Dir:        <PROJECT_REVIEWS_DIR>/results/codeql/YYYY-MM-DD/
SARIF:             <liste vorhandener SARIF im Result Dir oder explizit angegebene Quellen> | fehlt
```

CLI-Ermittlung:

- Zuerst `codeql version` pruefen.
- Wenn nicht im PATH, pruefe `k-playbook/codeql-cli/codeql/codeql version`.
- Wenn lokale DB `true` oder `planned` ist und keine CLI gefunden wird: abbrechen und `/k-install-codeql` nennen.
- Wenn nur GitHub CodeQL aktiv ist und keine lokale CLI vorhanden ist: nicht abbrechen, solange SARIF bereits vorhanden ist oder User nur einen bestehenden GitHub-Code-Scanning-Export bewerten will. Sonst `/k-install-codeql --cli-only` empfehlen.

## Ausfuehrungsentscheidung

Frage vor langlaufenden Operationen, was ausgefuehrt werden soll. Nicht automatisch Datenbanken erzeugen oder Analysen starten.

Optionen:

- **Vorhandene SARIF auswerten (Default)**: Nutzt SARIF aus `k-playbook/reviews/results/codeql/<datum>/raw/` oder explizit vom User angegebene SARIF-Dateien. Keine neue Analyse.
- **Lokale CodeQL-Analyse ausfuehren**: Nur erlaubt, wenn `local-database:` `true` oder `planned` ist und CLI + Datenbankpfad vorhanden sind. Vor Ausfuehrung exakte Befehle zeigen und bestaetigen lassen.
- **Nur Preflight**: Keine Report-Erzeugung, nur Status zeigen.
- **Abbrechen**.

Wenn lokale Analyse ausgefuehrt wird:

- Nutze `languages:` aus dem CodeQL-Block als Default.
- Nutze `queries:` aus dem CodeQL-Block als Default.
- Nutze `target:` aus dem CodeQL-Block als Analyse-/Kontextwurzel. Wenn `target:` fehlt, gilt legacy-kompatibel `.`.
- Nutze `database:` als Datenbankbasis.
- Schreibe SARIF nach `k-playbook/reviews/results/codeql/YYYY-MM-DD/raw/codeql-<language>.sarif`.
- Keine SARIF-Uploads.
- Keine GitHub-Actions-Aenderungen.
- Keine Code-Aenderungen.

## Bewertungskriterien

Erzeuge immer zwei Ebenen der Bewertung:

1. **Tool-Bewertung** aus SARIF:
   - `security-severity`
   - `problem.severity`
   - `precision`
   - CWE-/CVE-Tags
   - Regel-ID und Anzahl

2. **Review-Bewertung** durch Code-/Kontextsichtung:
   - `confirmed` / belastbarer echter Befund
   - `context-needed` / Kontext oder Datenfluss muss weiter geprueft werden
   - `likely-false-positive` / wahrscheinlich Fehlalarm
   - `accepted` / bekannte bewusste Entscheidung, falls von `known-decisions.md` gedeckt

Priorisierung:

- CVE-Tags zuerst, falls CodeQL konkrete CVE-Regeln meldet.
- Danach `security-severity >= 9.0`.
- Danach hohe Severity mit hoher Precision.
- Danach kleine, klar pruefbare Finding-Gruppen.
- Massenmeldungen mit mittlerer Precision batchweise behandeln, nicht als einzelne P1-Funde.

Wichtig: CodeQL meldet meistens CWE-basierte Code-Schwachstellenklassen, keine Dependency-CVEs. Dependency-CVEs gehoeren in eine separate Scan-Familie.

## Report-Artefakte und Pfadkonvention

`k-playbook/checks/` bleibt fuer ausfuehrbare Checks, Check-Skripte und Check-Definitionen reserviert. Review-Ergebnisse werden unter `k-playbook/reviews/` abgelegt.

Dieses Review schreibt in:

`k-playbook/reviews/results/codeql/YYYY-MM-DD/`

Dateien:

- `assessment.md` — zentrale kuratierte Bewertung.
- `findings.md` — vollstaendige Findings-Arbeitsliste, falls noch nicht vorhanden oder wenn SARIF neu ausgewertet wurde.
- `raw/codeql-<language>.sarif` — maschinenlesbare CodeQL-Rohdaten.

Optionale Kompatibilitaet beim Auswerten bestehender Projekte:

- Wenn alte SARIF-Dateien unter `checks:` liegen, duerfen sie als Quelle gelesen werden.
- Neue oder neu erzeugte SARIF-Dateien werden aber in `k-playbook/reviews/results/codeql/YYYY-MM-DD/raw/` geschrieben.

Typische Rohdateien:

- `codeql-python.sarif`
- `codeql-javascript-typescript.sarif`
- weitere `codeql-<language>.sarif`

## Assessment-Format

`assessment.md` enthaelt mindestens:

```markdown
# CodeQL Assessment - YYYY-MM-DD

## Quellen

- SARIF: `raw/codeql-...sarif`
- Finding-Register: `findings.md`

## Kurzfazit

- CVE-bezogene Findings: <anzahl / keine>
- CodeQL meldet hier <CWE-Code-Findings / konkrete CVE-Findings>.
- Abgrenzung: Dependency-CVEs werden separat gescannt.

## Kurzfazit CodeQL (manuelle Ersteinschaetzung)

- <N> Dateien / Sprachen / Workflows analysiert.
- Aus <N> Rohmeldungen ergeben sich nach erster Sichtung <N> vorlaeufig echte/plausible Befunde, <N> Kontextbedarf, <N> wahrscheinliche Fehlalarme.
- P1: <wichtigster belastbarer Befund mit Ort und Begruendung>.
- P2: <naechste Befundgruppe>.
- Zusaetzlich zu pruefen: <Kontextbedarf>.

## Regelbewertung

| Prioritaet | Regel | Anzahl | Security Severity | Problem Severity | Precision | CWE/CVE | Bewertung |
|---|---:|---:|---:|---|---|---|---|

## Sofortige Triage-Reihenfolge

1. ...

## Top-Findings Nach Prioritaet

### P01 `<rule>`

- Sprache: ...
- Ort: `path:line`
- Security Severity: ...
- CWE/CVE: ...
- Message: ...
- Review-Bewertung: `confirmed | context-needed | likely-false-positive | accepted`
- Begruendung: ...
- Naechster Schritt: Remediation spaeter / Known Decision / weiterer Kontext
```

## Finding-Register-Format

`findings.md` enthaelt alle SARIF-Results einzeln mit Statusfeld:

```markdown
#### <rule>-NNN

- Status: `open`
- Ort: `path:line`
- Message: ...
- Tool-Bewertung: severity/precision/CWE
- Review-Bewertung: _offen_
- Triage-Notiz: _offen_
```

## Was als belastbarer CodeQL-Fund zaehlt

- Der Datenfluss oder die API-Nutzung ist im Code nachvollziehbar.
- Die Quelle ist user-, tenant-, request- oder extern beeinflusst.
- Die Senke ist sicherheitsrelevant, z. B. HTTP-Request, HTML/DOM, Dateipfad, SQL/Command, Redirect, Fehlerausgabe, Log-Ausgabe mit Kontrollzeichenrisiko.
- Bestehende Validierung ist sichtbar unzureichend oder nicht auf dem relevanten Pfad.
- Der Befund ist nicht durch `known-decisions.md` akzeptiert.

## Was nicht als belastbarer Fund zaehlt

- Reine Tool-Meldung ohne nachvollziehbaren Datenfluss.
- Generierter, vendored oder bewusst ausgeschlossener Code.
- Test-/Fixture-Code ohne Produktpfad.
- Framework-Muster, die bereits durch zentrale Middleware/Helper sicher gemacht werden.
- Bekannte bewusste Entscheidung, die in `known-decisions.md` dokumentiert ist.

## Handoff

Nach Abschluss nennt `/k-review`:

```text
/k-remediation k-playbook/reviews/results/codeql/YYYY-MM-DD/assessment.md
```

Remediation ist ausdruecklich nicht Teil dieses Reviews.
