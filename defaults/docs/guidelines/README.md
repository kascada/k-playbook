---
title: "Guidelines"
description: "How the code and artifacts of this project should look — one document per topic, each rule a statement that can be checked against the existing code, with the same front matter as every document under docs/."
---

# Guidelines

A guideline describes the target state of the result: how the code and the artifacts of this
project should look, for example "design values live in `:root`" or "a command file is built
like this". It applies at all times and to everyone who changes the project, and it can be
checked against the existing code without looking at a diff.

What belongs elsewhere:

- What a session should **do** — update the docs in the same pass, where to put files, who
  decides — is an instruction, not a guideline.
- How to **run, build or inspect** something is a guide in `docs/manual/`. A guideline links to
  it instead of carrying the steps.

## One document per topic

- One topic per document, named `<topic>.md` in lower case with hyphens.
- Content is in English, like everything under `docs/`.
- What belongs to an existing guideline is added there, not placed next to it as a second
  document. Two documents on the same topic contradict each other sooner or later.

## Front matter

The same as every document under `docs/`: `title` and a one-sentence `description` are
required. `questions` lists the questions the guideline answers; they weigh high in search.
`sources` names what the guideline governs and what enforces it, such as the stylesheet and
its test. `generated` is absent, and `verified` is set only by a person.

## Body

- A short opening: what the guideline covers and where the values it governs live.
- One section per rule. A rule is a statement about the result that can be checked; add the
  reason in a sentence when it is not obvious.
- If a test or check enforces a rule, name it and say that it is not weakened to make a
  change pass.

How sessions are pointed to the guidelines that concern their change is not defined yet; until
then, the front matter above is complete.

## Changing guidelines

Existing documents are changed with the document operations of k-playbook,
`k_playbook_doc_replace` and `k_playbook_doc_append` (`k-playbook doc replace|append`); new
ones are created with Kephalaion `create`.

k-playbook writes this README from the template `k-playbook:/docs/guidelines/README.md`;
changes made here by hand are overwritten.
