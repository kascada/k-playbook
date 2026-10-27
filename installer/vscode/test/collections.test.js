'use strict';

const assert = require('node:assert/strict');
const { test } = require('node:test');

const { readCollections } = require('../lib/collections');

// fakeRun antwortet wie execFile mit den gegebenen Werten und hält den Aufruf
// fest.
function fakeRun(error, stdout, stderr = '') {
	const calls = [];
	const run = (program, args, options, callback) => {
		calls.push({ program, args, options });
		callback(error, stdout, stderr);
	};
	return { run, calls };
}

test('ruft das Programm im Projektordner und gibt seine Antwort zurück', async () => {
	const answer = { configured: true, folders: [{ uri: 'keph://vm/k-playbook', name: 'Keph vm:k-playbook' }] };
	const { run, calls } = fakeRun(null, JSON.stringify(answer));

	assert.deepEqual(await readCollections('/bin/k-playbook', '/projekt', run), answer);
	assert.equal(calls[0].program, '/bin/k-playbook');
	assert.deepEqual(calls[0].args, ['vscode', 'collections', '--json']);
	assert.equal(calls[0].options.cwd, '/projekt');
	assert.ok(calls[0].options.timeout > 0, 'ohne Frist');
});

test('ein scheiternder Aufruf nennt stderr', async () => {
	const { run } = fakeRun(new Error('Exit 1'), '', 'unbekanntes Kommando: vscode collections\n');

	await assert.rejects(readCollections('/bin/k-playbook', '/projekt', run), /unbekanntes Kommando/);
});

test('eine unlesbare oder falsch geformte Antwort wirft', async () => {
	await assert.rejects(readCollections('k', '/p', fakeRun(null, 'kein json').run), /kein JSON/);
	await assert.rejects(readCollections('k', '/p', fakeRun(null, '{"configured":true}').run), /erwartete Form/);
});
