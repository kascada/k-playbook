---
title: "Version inventory"
description: "The version inventory of the project, every declared version of packages, tools, runtimes, container images and Helm charts with its origin, written by k-playbook inventory."
---

# Version inventory

`inventory.md` lists the **declared** versions of the project — packages, tools, runtimes,
container images, Helm charts and their dependencies —, separated by environment and with the
origin of each line; not what is active at run time. Its counts are in the front matter under
`generated.stats`.

It is written by the program, `k-playbook inventory` (command `/k-doc-inventory`), from the
declarative sources of the project, and only when its content changes. Which sources are read
beyond the ones found by default, and which are excluded, is set in the front matter of
`keph:/k-playbook/inventory.md` in the project collection, not in `hints`; only paths below the
project root are read.
Changes made to the document by hand are lost on the next run.

The profiles in `generated/libs/` build on this inventory.

k-playbook writes this README from the template `k-playbook:/docs/generated/versions/README.md`;
changes made here by hand are overwritten.
