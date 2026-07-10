# Tech-Debt Review

> **Vor dem Start:** [`known-decisions.md`](known-decisions.md) lesen — alle dort aufgeführten Punkte gelten als bekannt und erledigt, nicht als Findings melden.

## Schritt 1: Tech-Debt-Analyse

/engineering:tech-debt

Analysiere die Quell- und Infrastruktur-Verzeichnisse des Projekts — schließe `priv/`, `secure/`, `tasks/` und virtuelle Umgebungen aus.
Kategorisiere und priorisiere alle Tech-Debt-Kandidaten.
Keine Code-Änderungen. Schreibe das vollständige Ergebnis als Markdown nach `priv/review/result-tech.md`.

---

## Schritt 2: Audit-Befunde abarbeiten

> Neuen Chat starten, dann:
> `/k-remediation priv/review/result-tech.md`
