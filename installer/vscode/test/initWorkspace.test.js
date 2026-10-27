'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { test, beforeEach, afterEach } = require('node:test');

const { state, reset, vscode } = require('./stub');
const action = require('../actions/initWorkspace');

const kephalaion = 'kascada.kephalaion';
const oeffentlich = { uri: 'keph://vm/k-playbook', name: 'Keph vm:k-playbook' };
const projekt = { uri: 'keph://vm/omni', name: 'Keph vm:omni' };

const echteDeps = { ...action.deps };
let dir;
let aufrufe;

// einrichten gibt jedem Test einen eigenen Projektordner, eine vorhandene
// Kephalaion-Erweiterung und ein Programm, das mit answer antwortet.
function einrichten(answer = { configured: true, folders: [oeffentlich, projekt] }) {
	state.extensions[kephalaion] = { id: kephalaion };
	state.workspaceFolders = [{ name: path.basename(dir), uri: vscode.Uri.file(dir) }];
	action.deps.resolveExecutable = () => '/bin/k-playbook';
	action.deps.readCollections = async (program, cwd) => {
		aufrufe.push({ program, cwd });
		if (answer instanceof Error) {
			throw answer;
		}
		return answer;
	};
}

const keph = (folder) => ({ name: folder.name, uri: vscode.Uri.parse(folder.uri) });
const geoeffnet = () => state.executedCommands.filter((args) => args[0] === 'vscode.openFolder');

beforeEach(() => {
	reset();
	aufrufe = [];
	dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kp-ws-'));
});

afterEach(() => {
	Object.assign(action.deps, echteDeps);
	fs.rmSync(dir, { recursive: true, force: true });
});

test('ohne Projektordner bleibt der Start still, von Hand gibt es eine Meldung', async () => {
	await action.onStartup();
	assert.deepEqual([state.infoMessages, state.warningMessages], [[], []]);

	await action.run();
	assert.match(state.infoMessages[0], /Kein Projektordner/);
});

test('ohne Kephalaion-Erweiterung wird nichts gefragt und nichts eingetragen', async () => {
	einrichten();
	delete state.extensions[kephalaion];

	await action.onStartup();
	assert.equal(aufrufe.length, 0);
	assert.deepEqual(state.infoMessages, []);

	await action.run();
	assert.match(state.infoMessages[0], /Kephalaion-Erweiterung/);
	assert.deepEqual([state.updates, geoeffnet()], [[], []]);
});

test('ohne Anbindung bleibt der Start still, von Hand steht der Grund da', async () => {
	einrichten({ configured: false, folders: [], message: 'keine Anbindung an Kephalaion' });

	await action.onStartup();
	assert.deepEqual([state.infoMessages, state.warningMessages], [[], []]);
	assert.equal(aufrufe[0].cwd, dir);

	await action.run();
	assert.match(state.infoMessages[0], /keine Anbindung an Kephalaion/);
});

test('ein scheiternder Aufruf wird beim Start als Warnung gemeldet, von Hand als Fehler', async () => {
	einrichten(new Error('k-playbook vscode collections: kaputt'));

	await action.onStartup();
	assert.match(state.warningMessages[0], /kaputt/);

	await action.run();
	assert.match(state.errorMessages[0].message, /kaputt/);
});

test('sind alle Collections eingebunden, wird nichts geändert', async () => {
	einrichten();
	state.workspaceFile = vscode.Uri.file(path.join(dir, 'x.code-workspace'));
	state.workspaceFolders.push(keph(oeffentlich), keph(projekt));

	await action.onStartup();
	assert.deepEqual([state.updates, geoeffnet(), state.warningMessages], [[], [], []]);
});

test('mit offener Workspace-Datei wird nur das Fehlende hinten angefügt', async () => {
	einrichten();
	state.workspaceFile = vscode.Uri.file(path.join(dir, 'x.code-workspace'));
	state.workspaceFolders.push(keph(oeffentlich));

	await action.onStartup();
	assert.equal(state.updates.length, 1);
	const { start, deleteCount, added } = state.updates[0];
	assert.deepEqual([start, deleteCount], [2, 0]);
	assert.deepEqual(added.map((folder) => [folder.uri.toString(), folder.name]), [[projekt.uri, projekt.name]]);
	assert.deepEqual([geoeffnet(), state.warningMessages, state.infoMessages], [[], [], []]);
});

test('lehnt VS Code das Einbinden ab, bleibt das nicht still', async () => {
	einrichten();
	state.workspaceFile = vscode.Uri.file(path.join(dir, 'x.code-workspace'));
	state.updateResult = false;

	await action.onStartup();
	assert.match(state.warningMessages[0], /abgelehnt/);
});

test('nur ein Ordner offen, keine Workspace-Datei: anlegen und öffnen', async () => {
	einrichten();

	await action.onStartup();
	const name = `${path.basename(dir)}.code-workspace`;
	const angelegt = JSON.parse(fs.readFileSync(path.join(dir, name), 'utf8'));
	assert.deepEqual(angelegt, {
		folders: [{ path: '.' }, { name: oeffentlich.name, uri: oeffentlich.uri }, { name: projekt.name, uri: projekt.uri }],
		settings: {},
	});
	assert.equal(geoeffnet().length, 1);
	assert.equal(geoeffnet()[0][1].toString(), `file://${dir}/${name}`);
	assert.deepEqual(state.updates, []);
});

test('eine vorhandene Workspace-Datei wird geöffnet, nicht überschrieben', async () => {
	einrichten();
	const vorhanden = path.join(dir, 'omni.code-workspace');
	fs.writeFileSync(vorhanden, '// meine Datei\n{ "folders": [{ "path": "." }] }\n');

	await action.onStartup();
	assert.equal(fs.readFileSync(vorhanden, 'utf8'), '// meine Datei\n{ "folders": [{ "path": "." }] }\n');
	assert.deepEqual(fs.readdirSync(dir), ['omni.code-workspace']);
	assert.equal(geoeffnet()[0][1].toString(), `file://${dir}/omni.code-workspace`);
});

test('bei mehreren Workspace-Dateien gilt die mit dem Ordnernamen, sonst keine', async () => {
	einrichten();
	fs.writeFileSync(path.join(dir, 'a.code-workspace'), '{}');
	fs.writeFileSync(path.join(dir, 'b.code-workspace'), '{}');

	await action.onStartup();
	assert.deepEqual(geoeffnet(), []);
	assert.match(state.warningMessages[0], /Mehrere Workspace-Dateien.*a\.code-workspace, b\.code-workspace/);

	const eigene = `${path.basename(dir)}.code-workspace`;
	fs.writeFileSync(path.join(dir, eigene), '{}');
	await action.onStartup();
	assert.equal(geoeffnet()[0][1].toString(), `file://${dir}/${eigene}`);
});
