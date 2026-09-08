# Orthography

The root `README.md` and everything under `docs/` are English and therefore outside
this rule. All German text in this repository uses correct orthography with umlauts
and ß — no ASCII transliteration.

| instead of                             | correct                       |
| -------------------------------------- | ----------------------------- |
| `fuer`, `ueber`, `pruefen`             | für, über, prüfen             |
| `Oberflaeche`, `Aenderung`, `naechste` | Oberfläche, Änderung, nächste |
| `heisst`, `schliessen`, `muessen`      | heißt, schließen, müssen      |

This applies to everything people read: documentation, interface text, commands,
skills, rules, review recipes, checks, commit descriptions, and code comments.

## Where ASCII remains

Not everything is text. These items remain ASCII:

- **File and directory names**, and consequently the catalog keys derived from them.
  macOS stores umlauts in decomposed form (NFD), Linux in composed form (NFC); the
  same name would no longer be the same string on two machines. Overlay matching
  compares precisely these names.
- **Configuration keys** in `K-PLAYBOOK.yaml`, command and skill names,
  identifiers in code, and branch names.

In short: what is read uses umlauts; what is compared or invoked does not.

## Why this is safe

- Go source files are UTF-8 according to the language specification.
- The interface declares `charset=utf-8`, and the file server supplies the same
  charset for `.js` and `.css`.
- Markdown is UTF-8 anyway; the documentation has long used `—`, `·`, and `→`.

The actual reason for this decision is different:
**a half-completed transition is the worst state.** As long as both spellings
coexist, a search for "Auflösung" will not find occurrences of `Aufloesung`,
and vice versa.

## When writing new text

A search for the literal patterns `ae`, `oe`, and `ue` is unusable — they also occur
in correct German words ("neue", "Quelle", "aktuell", "Sequenz"). To check for
transliterations, search for the actual patterns instead:

```bash
rg -i 'fuer|ueber|pruef|moeglich|naechst|waehl|aender|zurueck|oeffn|loesch|Oberflaeche|heisst|schliess|muess|koenn|laesst|traegt'
```
