// Aktion „Kephalaion-Collections einbinden“: bindet die öffentliche
// Collection k-playbook und die des Projekts als Ordner in den Arbeitsbereich
// ein. Sie läuft bei jedem Fensterstart von selbst (onStartup) und ist von
// Hand aufrufbar (run).
//
// Drei Fälle:
//   - Alles eingebunden: nichts zu tun.
//   - Eine Workspace-Datei ist offen, es fehlt etwas: nachtragen. VS Code
//     schreibt die Ordner selbst in genau die geöffnete Datei — welche das
//     ist, muss hier niemand wissen.
//   - Nur ein Ordner ist offen: Ordner nachzutragen ergäbe einen unbenannten
//     Workspace, den VS Code beim Schließen verwirft. Stattdessen wird die
//     Workspace-Datei des Ordners geöffnet, nötigenfalls zuerst angelegt. Das
//     Fenster lädt dabei neu; den Rest erledigt der nächste Start.
//
// Eine bewusst entfernte Collection kommt beim nächsten Start zurück. Das ist
// bekannt und vorerst so gewollt.
'use strict';

const fs = require('node:fs');
const path = require('node:path');

const vscode = require('vscode');

const { readCollections } = require('../lib/collections');
const { resolveExecutable } = require('../lib/executable');

const id = 'kPlaybook.initWorkspace';

// kephalaionId ist die Erweiterung, die keph:// öffnet. Ohne sie wird nichts
// eingetragen: Es stünden Ordner da, die keiner öffnen kann.
const kephalaionId = 'kascada.kephalaion';

const program = 'k-playbook';

// deps sind die Wege nach außen, austauschbar, damit der Test nicht das
// Programm dieses Rechners ruft.
const deps = {
	resolveExecutable: (name) => resolveExecutable(name, process.env),
	readCollections,
};
const workspaceSuffix = '.code-workspace';

// Beim Start bleiben Zustände still, die keine Störung sind (kein Projekt,
// keine Anbindung, keine Kephalaion-Erweiterung); laut wird nur ein
// Fehlschlag. Von Hand gerufen bekommt jeder Ausgang eine Meldung, sonst
// drückt der Nutzer auf einen Befehl, der scheinbar nichts tut.
function reporter(manual) {
	return {
		async note(text) {
			if (manual) {
				await vscode.window.showInformationMessage(text);
			}
		},
		async fail(text) {
			if (manual) {
				await vscode.window.showErrorMessage(text);
			} else {
				await vscode.window.showWarningMessage(`k-playbook: ${text}`);
			}
		},
	};
}

// chooseWorkspaceFile wählt die Workspace-Datei im Projektordner: keine →
// <ordnername>.code-workspace wird angelegt; genau eine → diese; mehrere →
// die mit dem Ordnernamen, sonst keine, und error sagt warum.
function chooseWorkspaceFile(dir) {
	const own = `${path.basename(dir)}${workspaceSuffix}`;
	const present = fs
		.readdirSync(dir)
		.filter((name) => name.endsWith(workspaceSuffix))
		.sort();
	if (present.length === 0) {
		return { name: own, create: true };
	}
	if (present.length === 1) {
		return { name: present[0], create: false };
	}
	if (present.includes(own)) {
		return { name: own, create: false };
	}
	return {
		error:
			`Mehrere Workspace-Dateien im Projektordner (${present.join(', ')}). ` +
			'Öffne die gewünschte über „File > Open Workspace from File“; beim nächsten Start werden die Collections dort eingebunden.',
	};
}

// workspaceContent ist eine neue Workspace-Datei: der Projektordner und die
// Collections, in der Reihenfolge des Programms.
function workspaceContent(folders) {
	const content = {
		folders: [{ path: '.' }, ...folders.map((folder) => ({ name: folder.name, uri: folder.uri }))],
		settings: {},
	};
	return `${JSON.stringify(content, null, '\t')}\n`;
}

async function ensure(manual) {
	const report = reporter(manual);
	const folders = vscode.workspace.workspaceFolders || [];

	// Der Projektordner ist der erste Ordner auf der Platte; keph://-Ordner
	// und andere Schemata zählen nicht.
	const projectFolder = folders.find((folder) => folder.uri.scheme === 'file');
	if (!projectFolder) {
		await report.note('Kein Projektordner im Arbeitsbereich. Öffne zuerst einen Ordner.');
		return;
	}
	if (!vscode.extensions.getExtension(kephalaionId)) {
		await report.note(
			`Die Kephalaion-Erweiterung (${kephalaionId}) ist nicht installiert. Ohne sie lassen sich keine Collections öffnen.`,
		);
		return;
	}
	const executable = deps.resolveExecutable(program);
	if (!executable) {
		await report.note(`Programm nicht gefunden: ${program}. Erwartet wird es im PATH.`);
		return;
	}

	let answer;
	try {
		answer = await deps.readCollections(executable, projectFolder.uri.fsPath);
	} catch (error) {
		await report.fail(error.message);
		return;
	}
	if (!answer.configured) {
		await report.note(`Keine Collections einzubinden: ${answer.message}`);
		return;
	}

	const present = new Set(folders.map((folder) => folder.uri.toString()));
	const missing = answer.folders.filter(
		(folder) => !present.has(vscode.Uri.parse(folder.uri).toString()),
	);
	const names = (list) => list.map((folder) => folder.name).join(', ');
	if (missing.length === 0) {
		await report.note(`Alle Collections sind eingebunden: ${names(answer.folders)}.`);
		return;
	}

	// Eine offene Workspace-Datei, auch eine unbenannte: nachtragen. In einer
	// gespeicherten Datei hält VS Code das selbst fest.
	if (vscode.workspace.workspaceFile) {
		const added = vscode.workspace.updateWorkspaceFolders(
			folders.length,
			0,
			...missing.map((folder) => ({ uri: vscode.Uri.parse(folder.uri), name: folder.name })),
		);
		if (!added) {
			await report.fail(`VS Code hat das Einbinden abgelehnt: ${names(missing)}.`);
			return;
		}
		await report.note(`Eingebunden: ${names(missing)}.`);
		return;
	}

	const dir = projectFolder.uri.fsPath;
	let choice;
	try {
		choice = chooseWorkspaceFile(dir);
	} catch (error) {
		await report.fail(`Projektordner nicht lesbar: ${error.message}`);
		return;
	}
	if (choice.error) {
		await report.fail(choice.error);
		return;
	}
	if (choice.create) {
		try {
			// wx: Eine Datei, die inzwischen ein anderer angelegt hat, wird
			// nicht überschrieben.
			fs.writeFileSync(path.join(dir, choice.name), workspaceContent(answer.folders), { flag: 'wx' });
		} catch (error) {
			if (error.code !== 'EEXIST') {
				await report.fail(`${choice.name} nicht angelegt: ${error.message}`);
				return;
			}
		}
	}
	// joinPath statt Uri.file: so behält die Adresse Schema und Autorität des
	// Projektordners, auch in einer Remote-Umgebung.
	await vscode.commands.executeCommand('vscode.openFolder', vscode.Uri.joinPath(projectFolder.uri, choice.name));
}

async function run() {
	await ensure(true);
}

// onStartup läuft aus activate und darf nie werfen: Ein Fehlschlag wird
// gemeldet, die übrigen Aktionen bleiben verfügbar.
async function onStartup() {
	try {
		await ensure(false);
	} catch (error) {
		await vscode.window.showWarningMessage(
			`k-playbook: Collections nicht eingebunden: ${error && error.message ? error.message : error}`,
		);
	}
}

module.exports = { id, run, onStartup, chooseWorkspaceFile, workspaceContent, deps };
