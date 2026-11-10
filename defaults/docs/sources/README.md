---
title: "Sources outside the collection"
description: "One document per documentation source outside keph:/docs/, such as a docs/ directory in the repository, naming the source under sources and the name of its mirror under mirror."
---

# Sources outside the collection

Each document here stands for one source of documentation that does not live in the
collection, for example a directory in the repository. It names the source in the front
matter under `sources[].resource` (such as `project:/docs/`), the name of its mirror under
`mirror` — a single path segment — and briefly describes what lies there. This directory is
the list of these sources; the documents in it are maintained by hand.

k-playbook will mirror each source to `docs/generated/sources/<mirror>/`, where it is indexed
like every other document under `docs/`. The mirror is not built yet; until then a document
here is only the reference.

k-playbook writes this README from the template `k-playbook:/docs/sources/README.md`; changes
made here by hand are overwritten.
