'use strict';

const assert = require('node:assert/strict');
const { test, beforeEach } = require('node:test');

const { state, reset } = require('./stub');
const { pickFolder } = require('../lib/folders');

const ordner = (name) => ({ name, uri: { fsPath: `/tmp/${name}`, scheme: 'file' } });

beforeEach(reset);

test('ohne Ordner im Arbeitsbereich gibt es eine Meldung und keine Auswahl', async () => {
	state.workspaceFolders = [];

	assert.equal(await pickFolder(), null);
	assert.equal(state.folderPickCalls, 0);
	assert.equal(state.errorMessages.length, 1);
	assert.match(state.errorMessages[0].message, /Kein Ordner/);
});

test('bei genau einem Ordner wird nicht gefragt', async () => {
	const einziger = ordner('k-playbook');
	state.workspaceFolders = [einziger];

	assert.equal(await pickFolder(), einziger);
	assert.equal(state.folderPickCalls, 0);
	assert.deepEqual(state.errorMessages, []);
});

test('bei mehreren Ordnern entscheidet die Auswahl', async () => {
	const zweiter = ordner('zweites-projekt');
	state.workspaceFolders = [ordner('k-playbook'), zweiter];
	state.folderPick = zweiter;

	assert.equal(await pickFolder(), zweiter);
	assert.equal(state.folderPickCalls, 1);
	assert.ok(state.folderPickOptions.placeHolder);
});

test('ein Abbruch der Auswahl bleibt still', async () => {
	state.workspaceFolders = [ordner('a'), ordner('b')];
	state.folderPick = undefined;

	assert.equal(await pickFolder(), null);
	assert.equal(state.folderPickCalls, 1);
	assert.deepEqual(state.errorMessages, []);
});
