---
name: review-python-comment-hardspots
title: Python - Nicht-rekonstruierbare Entscheidungen kommentieren
language: python
interval-weeks: 16
scope-hint: Python-Quellen; Ausschluss: virtuelle Umgebungen, tests/fixtures
audit:
  enabled: true
  mode: evidence
  ruleIds:
    - hardspot-external-workaround
    - hardspot-order-dependency
    - hardspot-deliberate-deviation
    - hardspot-magic-value
    - hardspot-suppressed-error
    - hardspot-commented-out-code
    - hardspot-version-dependency
    - hardspot-remote-contract
  scope:
    paths:
      - "**/*.py"
      - "**/*.pyi"
review:
  enabled: true
---

# Review: Python-Comment-Hardspots

Finde in Python-Code Stellen, an denen selbst ein erfahrener Reviewer mit
Projektkenntnis nicht erkennen kann, warum der Code so geschrieben wurde wie er ist.

Der generische Ablauf wird von `/k-review` orchestriert. Dieses Rezept arbeitet in zwei
Betriebsarten: interaktiv über `/k-review`, wo der Grund geklärt und ein Kommentar
vorgeschlagen wird, und als Evidence-Quelle im Audit-Lauf (`audit.mode: evidence`), wo
dieselben Kriterien ohne Rückfrage angewendet werden. Die Prüfkriterien unten gelten in
beiden gleich; unterschiedlich ist allein die Ergebnisform am Ende der Datei.

## Zielgruppe des Codes

Sehr erfahrene Programmierer, die das Projekt kennen.

- Lokale Komplexität braucht keine Erklärung.
- Domänenwissen ist vorausgesetzt.
- Python-Idiome werden verstanden.

Es bleibt eine eng umrissene Restmenge: Stellen, an denen der Code anders aussieht als
die naheliegende Lösung und der Grund dafür nicht aus Code, Kontext oder Domänenwissen
ableitbar ist.

## Was als Fund zählt

Kandidaten sind Stellen, an denen die Antwort auf „warum nicht einfach so?" außerhalb
des sichtbaren Universums liegt und kein Kommentar sie nennt. Ein Fund braucht einen Ort
— Datei und Zeile —, eine Rule-ID aus der Liste unten und einen Satz, der sagt, welche
Frage der Code offen lässt.

## Was nicht als Fund zählt

- Alles, was ein erfahrener Reviewer aus dem Code selbst erschließen kann.
- Domänenlogik.
- Standard-Patterns.
- Performance-Optimierungen, deren Wirkung offensichtlich ist.
- Sicherheitsmaßnahmen, die als solche erkennbar sind.
- Stellen, an denen bereits ein Kommentar den Grund nennt — auch ein knapper.

Bei begründetem Zweifel: nicht aufnehmen. Eine leere Fundliste ist ein gültiges Ergebnis.

## Rule-IDs und `level`

Die Liste ist abschließend und deckt die typischen Fälle ab. Eine Rule-ID außerhalb
dieser Liste macht das Ergebnis ungültig — sie steht so auch in `audit.ruleIds` und wird
beim Melden geprüft.

| Rule-ID | Was der Fund zeigt | `level` |
|---|---|---|
| `hardspot-external-workaround` | Ein Workaround für einen Bug oder ein Quirk außerhalb dieses Codes, ohne Nennung der Ursache. | `note` |
| `hardspot-order-dependency` | Eine Reihenfolge-Abhängigkeit, die nicht aus dem Code folgt: Umstellen wäre erlaubt und ginge schief. | `warning` |
| `hardspot-deliberate-deviation` | Eine bewusste Abweichung vom Naheliegenden, deren Grund nicht dabeisteht. | `note` |
| `hardspot-magic-value` | Ein magischer Wert — Zahl, Grenze, Zeitspanne —, dessen Ursprung nicht erschließbar ist. | `note` |
| `hardspot-suppressed-error` | Ein unterdrückter oder pauschal abgefangener Fehler mit nicht-offensichtlichem Grund. | `warning` |
| `hardspot-commented-out-code` | Auskommentierter Code, der offenbar bewusst stehen bleiben soll, ohne dass dabeisteht, wofür. | `note` |
| `hardspot-version-dependency` | Ein nicht-portierbares oder versionsabhängiges Konstrukt, ohne Nennung der Version oder Plattform. | `note` |
| `hardspot-remote-contract` | Ein stiller Vertrag mit Code an anderer Stelle: Änderungen hier verlangen Änderungen dort. | `warning` |

`level` ist die einzige Wertung, die dieses Rezept vergibt, und es steht so im Ergebnis,
wie es hier festgelegt ist:

- `note` ist verbindlich und der Regelfall. Ein fehlender Kommentar ist kein Defekt,
  sondern eine Beobachtung; der Merge übernimmt den Wert unverändert
  (`severitySource: native`).
- `warning` ist ausdrücklich **kein** Urteil, sondern die Übergabe an das
  Severity-Mapping des Projekts (`scripts/severity.tsv`). Es steht bei den drei Fällen,
  hinter denen ein echter Fehler stecken kann statt nur einer offenen Frage. Fehlt dort
  ein Eintrag, bleibt es bei `warning`.
- `error` vergibt dieses Rezept nicht.

## Ausschlüsse

Die zentralen Ausschlüsse der Modulsuche — `k-playbook/`, `k-playbook-local/`, `vendor/`,
`node_modules/`, `testdata/` und Punkt-Verzeichnisse — gelten geerbt und stehen deshalb
nicht noch einmal in `scope.paths`. Zusätzlich bleiben außen vor:

- Virtuelle Umgebungen (`venv/`) und Build-Ausgaben (`dist/`, `build/`).
- Test-Fixtures und Beispieldaten (`tests/fixtures/`, `fixtures/`).
- Generierter Code (`*_pb2.py`, Migrationen, Stubs aus Werkzeugen).

## Anti-Muster

- Lokale Komplexität kommentieren.
- Domänenwissen kommentieren.
- Leere Liste vermeiden wollen.
- Vermutungen als Fakten formulieren.
- Das Was erklären statt das Warum.

## Ergebnisform in `/k-review` (interaktiv)

Je Fund wird der Grund geklärt — notfalls per Rückfrage — und ein konkreter Kommentar
vorgeschlagen. Nur dieser Teil schlägt Kommentartexte vor; im Audit-Lauf gibt es sie nicht.

Stil-Wahl beim Kommentar:

- Inline am Zeilenende für sehr kurze Hinweise an einer einzelnen Zeile oder Konstante.
- Einzeiliger Kommentar darüber, wenn der Hinweis kurz ist und sich auf wenige Zeilen
  bezieht.
- Block-Kommentar über dem Abschnitt, wenn mehrere Sätze nötig sind.
- Docstring nur, wenn die nicht-rekonstruierbare Entscheidung das Wesen der gesamten
  Funktion betrifft.

Qualitätskriterien für den vorgeschlagenen Kommentar:

- Erklärt das Warum, das man nicht erschließen kann.
- Nennt die externe Ursache konkret.
- Verweist auf Quellen, wo möglich.
- Ist so kurz wie möglich.
- Altert gut.
- Steht in der Sprache des umgebenden Codes.

## Ergebnisform im Audit-Lauf (`mode: evidence`)

Der Eintrag heißt `python-comment-hardspots` und schreibt genau ein Artefakt:
`raw/python-comment-hardspots.sarif`. Ein Markdown-Ergebnis, ein Family-Ordner oder ein
zweites `review-input.*` entstehen nicht. Den Ablauf beschreibt `/k-audit`, Schritt 5;
rezeptspezifisch ist:

- `tool.driver.name` ist `python-comment-hardspots` — der Eintragsname, nicht der
  Rezeptdateiname.
- Jeder Fund trägt eine `ruleId` aus der Liste oben, das dort festgelegte `level`, genau
  einen Fundort mit projektrelativem Pfad und Startzeile und eine `message.text` von
  einem Satz.
- Die `message.text` nennt die **offene Frage**, nicht einen Kommentarvorschlag: Ohne
  Rückfrage ist der Grund nicht geklärt, und ein geratener Kommentar wäre genau das
  Anti-Muster „Vermutungen als Fakten formulieren".
- Mehrere Instanzen desselben Falls in derselben Datei sind mehrere Funde. Fasse sie
  nicht selbst zusammen: der Merge bündelt sie zu einer Gruppe je Rule-ID und Datei, und
  ihre Zahl ist die Aussage, die die Triage daraus liest.
- Keine Priorität `P1`–`P3`, keine Kategoriebuchstaben `S`/`T`/`K`/`F`/`A`/`X`, keine
  Bündelung. Das ist Sache von `commands/_audit/review-scan-triage.md`.

**Mengendeckel: höchstens 25 Funde je Lauf.** Niedriger als bei einem Befund-Rezept, weil
jeder Fund hier eine Frage an einen Menschen ist: Was diese Funde auslösen, ist eine
Klärung mit dem, der den Code geschrieben hat, und davon sind fünfundzwanzig je Lauf schon
viel. Der Deckel ist eine Anweisung an dieses Rezept und wird beim Melden nicht erzwungen
— eine Kappung dort verwürfe Funde, die danach niemand mehr sieht.

Auswahlregel bei Überschreitung:

1. Gezählt wird der einzelne Fund, nicht die Gruppe.
2. Es entfallen **ganze** Bündel aus Rule-ID und Datei, nie einzelne Instanzen daraus.
   Eine gekürzte Instanzzahl wäre eine falsche Aussage über den Umfang.
3. Es bleibt stehen, was zuerst kommt: `warning` vor `note`; bei gleichem `level` das
   Bündel mit mehr Instanzen; bei gleicher Zahl alphabetisch nach Pfad, dann nach
   Rule-ID. Damit trifft ein zweiter Lauf über denselben Stand dieselbe Auswahl.
4. Nenne die Zahl der ausgelassenen Bündel und Funde im `reason` der Fertigmeldung. Ein
   stillschweigend gekürztes Ergebnis sähe aus wie ein vollständiges.
