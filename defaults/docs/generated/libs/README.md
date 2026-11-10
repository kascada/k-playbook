---
title: "Profiles of libraries and tools"
description: "Curated profiles of the non-trivial libraries and tools of the project, with pitfalls and idioms, one document per item of the version inventory, written by /k-docs-tools."
---

# Profiles of libraries and tools

Each document describes one library or tool from the version inventory
(`keph:/docs/generated/versions/inventory.md`) — its role in the project, version and
migration, pitfalls, idioms and anti-patterns; it does not replace the official
documentation. The inventory item and version it was read against are in the front matter
under `sources`; where the inventory differs by now, the inventory is right. Who wrote it is
under `generated.by` (`k-docs-tools`); `questions` lists the questions the document answers.

Only the `hints` in the front matter are maintained by hand: short sentences on what the next
run should watch for. Everything else is rewritten by the next run.

Documents are written via `k-playbook docs checkout libs` and `k-playbook docs publish libs`.
k-playbook writes this README from the template `k-playbook:/docs/generated/libs/README.md`;
changes made here by hand are overwritten.
