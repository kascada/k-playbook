'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { test, beforeEach, afterEach } = require('node:test');

const { state, reset, vscode } = require('./stub');
const openCode = require('../actions/openCode');

const ordner = (name) => ({ name, uri: { fsPath: `/tmp/${name}`, scheme: 'file' } });

let pathBefore;
let binVerzeichnis;

function programm(name) {
	fs.writeFileSync(path.join(binVerzeichnis, name), '#!/bin/sh\nexit 0\n', { mode: 0o755 });
	return path.join(binVerzeichnis, name);
}

beforeEach(() => {
	reset();
	binVerzeichnis = fs.mkdtempSync(path.join(os.tmpdir(), 'kplaybook-opencode-'));
	pathBefore = process.env.PATH;
	process.env.PATH = binVerzeichnis;
});

afterEach(() => {
	process.env.PATH = pathBefore;
});

test('das Terminal entsteht im Editor-Bereich, im gewählten Ordner, und wird gezeigt', async () => {
	const erwartet = programm('opencode-attach');
	const gewaehlt = ordner('k-playbook');
	state.workspaceFolders = [gewaehlt];

	await openCode.run();

	assert.equal(state.terminals.length, 1);
	const options = state.terminals[0].options;
	assert.equal(options.location, vscode.TerminalLocation.Editor);
	assert.equal(options.cwd, gewaehlt.uri);
	assert.equal(options.shellPath, erwartet);
	assert.deepEqual(options.shellArgs, []);
	assert.equal(options.name, 'OpenCode · k-playbook');
	assert.equal(state.terminals[0].shown, 1);
	assert.deepEqual(state.errorMessages, []);
});

test('bei mehreren Ordnern startet das Terminal im ausgewählten', async () => {
	programm('opencode-attach');
	const zweiter = ordner('anderes-projekt');
	state.workspaceFolders = [ordner('k-playbook'), zweiter];
	state.folderPick = zweiter;

	await openCode.run();

	assert.equal(state.terminals.length, 1);
	assert.equal(state.terminals[0].options.cwd, zweiter.uri);
	assert.equal(state.terminals[0].options.name, 'OpenCode · anderes-projekt');
});

test('ein Abbruch der Ordnerwahl öffnet kein Terminal', async () => {
	programm('opencode-attach');
	state.workspaceFolders = [ordner('a'), ordner('b')];
	state.folderPick = undefined;

	await openCode.run();

	assert.deepEqual(state.terminals, []);
	assert.deepEqual(state.errorMessages, []);
});

test('ohne Ordner im Arbeitsbereich öffnet kein Terminal, es gibt eine Meldung', async () => {
	programm('opencode-attach');
	state.workspaceFolders = [];

	await openCode.run();

	assert.deepEqual(state.terminals, []);
	assert.equal(state.errorMessages.length, 1);
});

test('ein falscher Pfad in der Einstellung ergibt die Meldung mit Knopf, kein Terminal', async () => {
	state.workspaceFolders = [ordner('k-playbook')];
	state.configuration['kPlaybook.openCode.executable'] = '/gibt-es-nicht/opencode-attach';

	await openCode.run();

	assert.deepEqual(state.terminals, []);
	assert.equal(state.errorMessages.length, 1);
	assert.match(state.errorMessages[0].message, /gibt-es-nicht/);
	assert.deepEqual(state.errorMessages[0].items, ['Einstellung öffnen']);
});

test('ein eingestelltes Programm und eingestellte Argumente werden übernommen', async () => {
	const eigenes = programm('eigener-attach');
	state.workspaceFolders = [ordner('k-playbook')];
	state.configuration['kPlaybook.openCode.executable'] = 'eigener-attach';
	state.configuration['kPlaybook.openCode.args'] = ['--model', 'opus', 7, null];

	await openCode.run();

	assert.equal(state.terminals[0].options.shellPath, eigenes);
	assert.deepEqual(state.terminals[0].options.shellArgs, ['--model', 'opus']);
});

test('ein Fehlschlag beim Anlegen des Terminals wird gemeldet', async () => {
	programm('opencode-attach');
	state.workspaceFolders = [ordner('k-playbook')];
	state.terminalError = new Error('kein Terminal frei');

	await openCode.run();

	assert.equal(state.errorMessages.length, 1);
	assert.match(state.errorMessages[0].message, /kein Terminal frei/);
});
