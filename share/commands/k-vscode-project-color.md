---
description: Create or update a project-specific VS Code workspace title and dark color theme in `.vscode/settings.json` based on the project path.
argument-hint: [project-name]
# model: github-copilot/gpt-5.4-mini
allowed-tools: [Read, Write, Bash, Glob]
---

# k-vscode-project-color

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Create or update a project-specific VS Code workspace configuration for easier window recognition.

- This skill is useful for making each workspace visually distinct, especially when switching windows with Alt+Tab.
- Use a different color/emoji for each project to avoid confusion.

Produces:
- `.vscode/settings.json` in the target project — created if missing, otherwise merged;
  unrelated existing settings are preserved.

## Schritt 1 — Zielprojekt bestimmen

- If the current working directory is inside a project folder, use that as the target.
- If the user provides an explicit project name or path, resolve it and validate that it exists.
- If the target cannot be determined, ask the user for the project root path.

## Schritt 2 — `.vscode/settings.json` sicherstellen

- Create the `.vscode` directory if needed.
- If `settings.json` exists, read and parse it as JSON.
- If it does not exist, start with an empty JSON object.

## Schritt 3 — Workspace-Gestaltung ergänzen oder aktualisieren

Write or merge the following keys into `.vscode/settings.json`:

```json
{
  "window.title": "📂 ${folderName}",
  "workbench.colorCustomizations": {
    "titleBar.activeBackground": "#001a4d",
    "titleBar.activeForeground": "#ffffff",
    "statusBar.background": "#001a4d"
  }
}
```

- Use a dark blue or deep color by default.
- If the user requested a specific project label, include that emoji or short text instead of `${folderName}`.
- Preserve any unrelated existing settings.

## Schritt 4 — Speichern und bestätigen

- Save the updated JSON to the project's `.vscode/settings.json` file.
- Tell the user that VS Code may need reload or restart for `window.title` to apply.

Folge-Command: keiner. Der Command endet mit dem Hinweis auf den Reload; mehr ist
nicht zu tun.
