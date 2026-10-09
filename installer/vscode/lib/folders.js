// Wahl des Arbeitsordners für eine Aktion.
'use strict';

const vscode = require('vscode');

// pickFolder gibt den Ordner zurück, in dem die Aktion laufen soll, oder null.
//
// Drei Fälle, und sie sind nicht dasselbe: ohne Ordner im Arbeitsbereich gibt
// es eine Meldung, weil der Nutzer sonst auf einen Befehl drückt, der nichts
// tut; bei genau einem Ordner wird nicht gefragt; bei mehreren entscheidet die
// Auswahl, und ihr Abbruch bleibt still — ein Abbruch ist keine Störung.
async function pickFolder() {
	const folders = vscode.workspace.workspaceFolders || [];

	if (folders.length === 0) {
		await vscode.window.showErrorMessage(
			'Kein Ordner im Arbeitsbereich. Öffne zuerst einen Ordner oder einen Arbeitsbereich.',
		);
		return null;
	}

	if (folders.length === 1) {
		return folders[0];
	}

	const chosen = await vscode.window.showWorkspaceFolderPick({
		placeHolder: 'In welchem Ordner soll OpenCode starten?',
	});
	return chosen || null;
}

module.exports = { pickFolder };
