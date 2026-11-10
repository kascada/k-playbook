---
title: "Manual documentation"
description: "Documentation written by hand, such as guides, background and decisions that no tool derives from the code, with the same front matter as every document under docs/."
---

# Manual documentation

The documents here are written by hand, not by a producer. They carry the same front matter
as every document under `docs/`: `title` and a one-sentence `description` are required,
`questions` lists the questions a document answers and `sources` what it was written against;
`generated` is absent, and `verified` is set only by a person.

Existing documents are changed with the document operations of k-playbook,
`k_playbook_doc_replace` and `k_playbook_doc_append` (`k-playbook doc replace|append`); new
ones are created with Kephalaion `create`. No producer writes here.

k-playbook writes this README from the template `k-playbook:/docs/manual/README.md`; changes
made here by hand are overwritten.
