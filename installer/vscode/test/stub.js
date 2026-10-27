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
	workspaceFile: undefined,
	updateResult: true,
	updates: [],
	extensions: {},
	infoMessages: [],
	warningMessages: [],
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
	state.workspaceFile = undefined;
	state.updateResult = true;
	state.updates = [];
	state.extensions = {};
	state.infoMessages = [];
	state.warningMessages = [];
}

// Uri bildet nur nach, was die Aktionen brauchen: Schema, Pfad und eine
// stabile Zeichenkette zum Vergleich.
class Uri {
	constructor(scheme, authority, uriPath) {
		this.scheme = scheme;
		this.authority = authority;
		this.path = uriPath;
	}

	get fsPath() {
		return this.path;
	}

	toString() {
		return `${this.scheme}://${this.authority}${this.path}`;
	}

	static file(fsPath) {
		return new Uri('file', '', fsPath);
	}

	static parse(value) {
		const match = /^([a-z][a-z0-9+.-]*):\/\/([^/]*)(.*)$/.exec(value);
		if (!match) {
			throw new Error(`Uri.parse: ${value}`);
		}
		return new Uri(match[1], match[2], match[3]);
	}

	static joinPath(base, ...segments) {
		return new Uri(base.scheme, base.authority, [base.path, ...segments].join('/'));
	}
}

const vscode = {
	Uri,
	TerminalLocation: { Panel: 1, Editor: 2 },
	window: {
		async showErrorMessage(message, ...items) {
			state.errorMessages.push({ message, items });
			return state.errorAnswer;
		},
		async showInformationMessage(message) {
			state.infoMessages.push(message);
		},
		async showWarningMessage(message) {
			state.warningMessages.push(message);
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
		get workspaceFile() {
			return state.workspaceFile;
		},
		updateWorkspaceFolders(start, deleteCount, ...added) {
			state.updates.push({ start, deleteCount, added });
			return state.updateResult;
		},
		getConfiguration(section) {
			return {
				get(key) {
					return state.configuration[`${section}.${key}`];
				},
			};
		},
	},
	extensions: {
		getExtension(extensionId) {
			return state.extensions[extensionId];
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
