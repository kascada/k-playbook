---
title: "Generated documentation"
description: "Documentation written by tools, one directory per producer; the only part maintained by hand is the hints in a document's front matter."
---

# Generated documentation

Every directory here belongs to one producer, a tool that writes its documents:

- `code/` — documentation from the source code, written by `/k-docs-code` and the skill
  `ks-overlay-repo-analyse`.
- `libs/` — profiles of the libraries and tools of the project, written by `/k-docs-tools`.
- `versions/` — the version inventory, written by `k-playbook inventory` (`/k-doc-inventory`).

Each document names its writer in the front matter under `generated.by` and the time of the
run under `generated.at`. The only part maintained by hand is `hints`: short sentences on what
the next run should watch for. Everything else is rewritten by the next run of the producer;
changes made by hand are lost.

k-playbook writes this README from the template `k-playbook:/docs/generated/README.md`;
changes made here by hand are overwritten.
