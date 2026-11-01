---
name: ks-overlay-repo-analyse
description: Use when the user wants to systematically understand a project that uses the Docker-Overlay-Pattern (Base-Image + slim Overlay-Repo that only contains delta files). Extracts or clones the base, diffs against the overlay, quantifies actual changes, writes structured docs that separate "base behavior" from "overlay additions". Trigger keywords - "Overlay", "Base-Image analysieren", "was ist wirklich neu", "FROM ghcr.io", "Custom-Image", "Delta zwischen Repos", "was macht der Overlay wirklich".
---

# Skill: Overlay-Repo-Analyse

Der Text dieses Skills steht unter `k-playbook:/commands/_skills/overlay-repo-analyse.md`.
Lies ihn nach der Verweisregel aus dem Briefing — MCP `k_playbook_doc_read` mit dem
Verweis als `ref`, ersatzweise
`k-playbook read k-playbook:/commands/_skills/overlay-repo-analyse.md` in der Shell — und
befolge ihn. Diese Datei selbst enthält sonst nichts.
