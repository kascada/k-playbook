---
title: "Project handbook"
description: "The handbook of this project for people, shown in the k-playbook interface under its own menu entry, one Markdown page per topic below docs/manual/handbook/."
---

# Project handbook

This is the start page of the project handbook. The k-playbook interface shows the handbook
under its own menu entry, next to the handbook of k-playbook itself, and always shows this page
first. A project does not have to use it; as long as no page has been added, this page is all
the handbook contains.

## What belongs here

The handbook is written for people who use or run the project: how to get started, how the
parts fit together, how to do the everyday tasks. It is the part of the manual documentation
that a reader opens in the interface and reads from top to bottom.

The rest of `docs/manual/` is written for sessions as much as for people — background,
decisions, guides that a session finds through search. A document that a person should be able
to read in the interface goes here; anything else stays one level up in `docs/manual/`.

## Adding a page

- Create one Markdown file per topic below `keph:/docs/manual/handbook/`, named `<topic>.md` in
  lower case with hyphens. Subdirectories are allowed.
- Give it the same front matter as every document under `docs/`: `title` and a one-sentence
  `description` are required.
- Write it in English, like everything under `docs/`.
- Link between handbook pages with relative links such as `[Setup](./setup.md)`.

New pages are created with Kephalaion `create`; existing ones are changed with the document
operations of k-playbook, `k_playbook_doc_replace` and `k_playbook_doc_append`
(`k-playbook doc replace|append`).

k-playbook writes this README from the template `k-playbook:/docs/manual/handbook/README.md`;
changes made here by hand are overwritten.
