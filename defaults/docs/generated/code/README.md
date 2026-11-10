---
title: "Documentation from the code"
description: "Project documentation generated from the source code, one document per topic, organised by meaning rather than by file, written by /k-docs-code and the skill ks-overlay-repo-analyse."
---

# Documentation from the code

Each document describes one topic of the code — purpose, flow, important places as
`path:line`, edge cases. Who wrote it is in the front matter under `generated.by`
(`k-docs-code` or `ks-overlay-repo-analyse`); `questions` lists the questions the document
answers.

Only the `hints` in the front matter are maintained by hand: short sentences on what the next
run should watch for. Everything else is rewritten by the next run.

Documents are written via `k-playbook docs checkout code` and `k-playbook docs publish code`.
k-playbook writes this README from the template `k-playbook:/docs/generated/code/README.md`;
changes made here by hand are overwritten.
