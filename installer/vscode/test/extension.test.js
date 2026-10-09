'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { test, beforeEach } = require('node:test');

const { state, reset } = require('./stub');
const extension = require('../extension');

const manifest = JSON.parse(
	fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'),
);

beforeEach(reset);

test('activate registriert jede Aktion genau einmal', () => {
	const context = { subscriptions: [] };

	extension.activate(context);

	const ids = state.registered.map((entry) => entry.id);
	assert.deepEqual(ids, extension.actions.map((action) => action.id));
	assert.equal(new Set(ids).size, ids.length);
	assert.equal(context.subscriptions.length, ids.length);
});

test('jede Aktion ist in package.json als Befehl eingetragen', () => {
	const beigetragen = manifest.contributes.commands.map((entry) => entry.command);
	for (const action of extension.actions) {
		assert.ok(
			beigetragen.includes(action.id),
			`${action.id} fehlt in contributes.commands`,
		);
		assert.equal(typeof action.run, 'function');
	}
});

test('jeder beigetragene Befehl hat eine Aktion', () => {
	const vorhanden = extension.actions.map((action) => action.id);
	for (const entry of manifest.contributes.commands) {
		assert.ok(vorhanden.includes(entry.command), `${entry.command} hat keine Aktion`);
		assert.equal(entry.category, 'k-playbook');
	}
});

test('das Manifest hält die Festlegungen der Auslieferung', () => {
	assert.equal(manifest.name, 'k-playbook-workspace-tools');
	assert.equal(manifest.publisher, 'kascada');
	assert.deepEqual(manifest.extensionKind, ['workspace']);
	// Ohne repository bricht vsce 4.0.0 den Bau der VSIX ab.
	assert.ok(manifest.repository, 'repository fehlt');
	// scope machine: kein Arbeitsbereich darf den ausgeführten Pfad umbiegen.
	for (const [key, property] of Object.entries(manifest.contributes.configuration.properties)) {
		assert.equal(property.scope, 'machine', `${key} hat nicht scope machine`);
	}
});
