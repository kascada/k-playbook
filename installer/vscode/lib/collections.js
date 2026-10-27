// Abfrage der Collections, die ein Fenster des Projekts eingebunden haben
// soll.
//
// Die Erweiterung leitet nichts selbst aus K-PLAYBOOK.local.yaml ab: Welche
// Collections dazugehören, weiß allein das Programm (`k-playbook vscode
// collections --json`). So steht die Regel an einer Stelle.
'use strict';

const { execFile } = require('node:child_process');

// timeoutMs begrenzt den Aufruf. Das Programm liest nur eine Datei und fragt
// keinen Node; die Frist ist nur gegen einen Aufruf, der hängt.
const timeoutMs = 10000;

// readCollections ruft das Programm im Projektordner und gibt seine Antwort
// zurück: { configured, folders: [{ uri, name }], message }. Ein Fehlschlag
// des Aufrufs oder eine unlesbare Antwort wirft. run ist execFile, als
// Parameter für den Test.
function readCollections(program, cwd, run = execFile) {
	return new Promise((resolve, reject) => {
		run(
			program,
			['vscode', 'collections', '--json'],
			{ cwd, timeout: timeoutMs },
			(error, stdout, stderr) => {
				if (error) {
					const detail = String(stderr || '').trim() || error.message;
					reject(new Error(`k-playbook vscode collections: ${detail}`));
					return;
				}
				let answer;
				try {
					answer = JSON.parse(stdout);
				} catch {
					reject(new Error('k-playbook vscode collections: Antwort ist kein JSON'));
					return;
				}
				if (!answer || typeof answer.configured !== 'boolean' || !Array.isArray(answer.folders)) {
					reject(new Error('k-playbook vscode collections: Antwort hat nicht die erwartete Form'));
					return;
				}
				resolve(answer);
			},
		);
	});
}

module.exports = { readCollections };
