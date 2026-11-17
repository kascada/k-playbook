'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { test } = require('node:test');

const { resolveExecutable } = require('../lib/executable');

// binDir legt ein Verzeichnis mit einem ausführbaren und einem nicht
// ausführbaren Eintrag an. Mehr braucht die Auflösung nicht.
function binDir() {
	const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kplaybook-exec-'));
	fs.writeFileSync(path.join(dir, 'ein-programm'), '#!/bin/sh\nexit 0\n', { mode: 0o755 });
	fs.writeFileSync(path.join(dir, 'nur-text'), 'kein Programm\n', { mode: 0o644 });
	fs.mkdirSync(path.join(dir, 'ein-verzeichnis'), { mode: 0o755 });
	return dir;
}

test('ein Name ohne Trenner wird im PATH gefunden', () => {
	const dir = binDir();
	const found = resolveExecutable('ein-programm', {
		PATH: ['/gibt-es-nicht', dir].join(path.delimiter),
	});
	assert.equal(found, path.join(dir, 'ein-programm'));
});

test('ein Name, der nirgends im PATH liegt, ergibt null', () => {
	assert.equal(resolveExecutable('ein-programm', { PATH: '/gibt-es-nicht' }), null);
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
	const direct = path.join(dir, 'ein-programm');
	assert.equal(resolveExecutable(direct, { PATH: '/gibt-es-nicht' }), direct);
	assert.equal(resolveExecutable(path.join(dir, 'fehlt-hier'), { PATH: dir }), null);
});

test('leer oder nur Leerzeichen ergibt null', () => {
	assert.equal(resolveExecutable('', { PATH: '/usr/bin' }), null);
	assert.equal(resolveExecutable('   ', { PATH: '/usr/bin' }), null);
	assert.equal(resolveExecutable(undefined, { PATH: '/usr/bin' }), null);
});
