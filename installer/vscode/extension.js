// Einstiegspunkt der Erweiterung „k-playbook Workspace Tools“.
//
// Die Erweiterung ist ein Sammelpaket: jede Aktion liegt als eigenes Modul
// unter actions/ und exportiert { id, run }. Hier wird nur registriert. Eine
// neue Aktion kommt dazu, indem sie in actions/ angelegt, in dieser Liste
// genannt und in package.json unter contributes.commands eingetragen wird —
// am Bestand ist dafür nichts zu ändern.
'use strict';

const vscode = require('vscode');

const actions = [require('./actions/openCode')];

function activate(context) {
	for (const action of actions) {
		context.subscriptions.push(
			vscode.commands.registerCommand(action.id, (...args) => action.run(...args)),
		);
	}
}

function deactivate() {}

module.exports = { activate, deactivate, actions };
