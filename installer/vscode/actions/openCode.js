// Aktion „OpenCode im neuen Tab“: startet das Anhäng-Programm von OpenCode in
// einem Terminal im Editor-Bereich, mit dem gewählten Workspace-Ordner als
// Arbeitsverzeichnis.
'use strict';

const vscode = require('vscode');

const { pickFolder } = require('../lib/folders');
const { resolveExecutable, reportMissing } = require('../lib/executable');

const id = 'kPlaybook.openCode';
const settingSection = 'kPlaybook.openCode';

function readArgs(config) {
	const args = config.get('args');
	if (!Array.isArray(args)) {
		return [];
	}
	return args.filter((entry) => typeof entry === 'string');
}

async function run() {
	const folder = await pickFolder();
	if (!folder) {
		return;
	}

	const config = vscode.workspace.getConfiguration(settingSection);
	const executable = String(config.get('executable') || 'opencode-attach').trim();

	// Der Pfad wird hier aufgelöst, nicht dem Terminal überlassen: siehe
	// lib/executable.js. Fehlt das Programm, endet die Aktion mit einer
	// Meldung und öffnet kein Terminal, das sofort wieder zuklappt.
	const shellPath = resolveExecutable(executable, process.env);
	if (!shellPath) {
		await reportMissing(executable, `${settingSection}.executable`);
		return;
	}

	try {
		const terminal = vscode.window.createTerminal({
			name: `OpenCode · ${folder.name}`,
			cwd: folder.uri,
			shellPath,
			shellArgs: readArgs(config),
			location: vscode.TerminalLocation.Editor,
		});
		terminal.show();
	} catch (error) {
		// Was das Programm nach dem Start selbst schreibt, bleibt in seinem
		// Terminal; gelesen wird es hier nicht. Nur ein Fehlschlag beim Anlegen
		// des Terminals gehört in eine Meldung.
		await vscode.window.showErrorMessage(
			`Terminal für OpenCode konnte nicht geöffnet werden: ${error && error.message ? error.message : error}`,
		);
	}
}

module.exports = { id, run };
