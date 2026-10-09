'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { test, beforeEach } = require('node:test');

const { state, reset } = require('./stub');
const { resolveExecutable, reportMissing } = require('../lib/executable');

// binDir legt ein Verzeichnis mit einem ausführbaren und einem nicht
// ausführbaren Eintrag an. Mehr braucht die Auflösung nicht.
function binDir() {
	const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kplaybook-exec-'));
	fs.writeFileSync(path.join(dir, 'opencode-attach'), '#!/bin/sh\nexit 0\n', { mode: 0o755 });
	fs.writeFileSync(path.join(dir, 'nur-text'), 'kein Programm\n', { mode: 0o644 });
	fs.mkdirSync(path.join(dir, 'ein-verzeichnis'), { mode: 0o755 });
	return dir;
}

beforeEach(reset);

test('ein Name ohne Trenner wird im PATH gefunden', () => {
	const dir = binDir();
	const found = resolveExecutable('opencode-attach', {
		PATH: ['/gibt-es-nicht', dir].join(path.delimiter),
	});
	assert.equal(found, path.join(dir, 'opencode-attach'));
});

test('ein Name, der nirgends im PATH liegt, ergibt null', () => {
	assert.equal(resolveExecutable('opencode-attach', { PATH: '/gibt-es-nicht' }), null);
});

test('ein nicht ausführbarer Eintrag im PATH zählt nicht', () => {
	const dir = binDir();
	assert.equal(resolveExecutable('nur-text', { PATH: dir }), null);
});

test('ein Verzeichnis mit passendem Namen zählt nicht', () => {
	const dir = binDir();
	assert.equal(resolveExecutable('ein-verzeichnis', { PATH: dir }), null);
});

test('ein Wert mit Trenner wird als Pfad genommen, nicht im PATH gesucht', () => {
	const dir = binDir();
	const direct = path.join(dir, 'opencode-attach');
	assert.equal(resolveExecutable(direct, { PATH: '/gibt-es-nicht' }), direct);
	assert.equal(resolveExecutable(path.join(dir, 'fehlt-hier'), { PATH: dir }), null);
});

test('leer oder nur Leerzeichen ergibt null', () => {
	assert.equal(resolveExecutable('', { PATH: '/usr/bin' }), null);
	assert.equal(resolveExecutable('   ', { PATH: '/usr/bin' }), null);
	assert.equal(resolveExecutable(undefined, { PATH: '/usr/bin' }), null);
});

test('ein fehlendes Programm wird als Meldung gezeigt, nicht nur im Terminal', async () => {
	await reportMissing('opencode-attach', 'kPlaybook.openCode.executable');

	assert.equal(state.errorMessages.length, 1);
	const shown = state.errorMessages[0];
	assert.match(shown.message, /opencode-attach/);
	assert.deepEqual(shown.items, ['Einstellung öffnen']);
	assert.deepEqual(state.executedCommands, []);
});

test('der Knopf führt in die Einstellung', async () => {
	state.errorAnswer = 'Einstellung öffnen';
	await reportMissing('opencode-attach', 'kPlaybook.openCode.executable');

	assert.deepEqual(state.executedCommands, [
		['workbench.action.openSettings', 'kPlaybook.openCode.executable'],
	]);
});
