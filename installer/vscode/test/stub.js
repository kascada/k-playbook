// Nachbildung des Moduls `vscode` für die Tests.
//
// Muster: `Module._load` wird einmal umgebogen, bevor der Prüfling geladen
// wird. Gegen ein echtes VS Code wird nicht getestet — die Remote-CLI
// ignoriert `--extensions-dir` und würde in die laufende Umgebung schreiben.
'use strict';

const Module = require('node:module');

const state = {
	workspaceFolders: [],
	folderPick: undefined,
	folderPickOptions: null,
	folderPickCalls: 0,
	configuration: {},
	terminals: [],
	terminalError: null,
	errorMessages: [],
	errorAnswer: undefined,
	registered: [],
	executedCommands: [],
};

function reset() {
	state.workspaceFolders = [];
	state.folderPick = undefined;
	state.folderPickOptions = null;
	state.folderPickCalls = 0;
	state.configuration = {};
	state.terminals = [];
	state.terminalError = null;
	state.errorMessages = [];
	state.errorAnswer = undefined;
	state.registered = [];
	state.executedCommands = [];
}

const vscode = {
	TerminalLocation: { Panel: 1, Editor: 2 },
	window: {
		async showErrorMessage(message, ...items) {
			state.errorMessages.push({ message, items });
			return state.errorAnswer;
		},
		async showWorkspaceFolderPick(options) {
			state.folderPickCalls += 1;
			state.folderPickOptions = options;
			return state.folderPick;
		},
		createTerminal(options) {
			if (state.terminalError) {
				throw state.terminalError;
			}
			const terminal = {
				options,
				shown: 0,
				show() {
					this.shown += 1;
				},
			};
			state.terminals.push(terminal);
			return terminal;
		},
	},
	workspace: {
		get workspaceFolders() {
			return state.workspaceFolders;
		},
		getConfiguration(section) {
			return {
				get(key) {
					return state.configuration[`${section}.${key}`];
				},
			};
		},
	},
	commands: {
		registerCommand(id, handler) {
			state.registered.push({ id, handler });
			return { dispose() {} };
		},
		async executeCommand(...args) {
			state.executedCommands.push(args);
		},
	},
};

const load = Module._load;
Module._load = function (request, ...rest) {
	if (request === 'vscode') {
		return vscode;
	}
	return load.call(this, request, ...rest);
};

module.exports = { state, reset, vscode };
