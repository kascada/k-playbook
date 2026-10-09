// Auflösung des zu startenden Programms.
//
// Aufgelöst wird hier, im Extension-Host, und `shellPath` bekommt den
// absoluten Pfad. Grund ist eine Messung: der Extension-Host trägt den aus
// der Login-Shell aufgelösten PATH (unter WSL einschließlich `~/.local/bin`
// und `~/.opencode/bin`), der `ptyHost`, der die Terminals startet, dagegen
// nicht. Ein bloßer Programmname als `shellPath` hängt damit an der Umgebung,
// die VS Code dem einzelnen Terminal mitgibt — ein absoluter Pfad nicht. Dazu
// kommt, dass ein fehlendes Programm so hier auffällt, wo eine Meldung
// angezeigt werden kann, und nicht erst im Terminal.
'use strict';

const fs = require('node:fs');
const path = require('node:path');

const vscode = require('vscode');

function isRunnableFile(candidate) {
	try {
		if (!fs.statSync(candidate).isFile()) {
			return false;
		}
		fs.accessSync(candidate, fs.constants.X_OK);
		return true;
	} catch {
		return false;
	}
}

// resolveExecutable gibt den absoluten Pfad des Programms zurück oder null.
// Ein Wert mit Pfadtrenner wird als Pfad genommen, alles andere im PATH
// gesucht. env ist ein Parameter, damit der Test ihn setzen kann.
function resolveExecutable(executable, env) {
	const wanted = String(executable || '').trim();
	if (wanted === '') {
		return null;
	}

	if (wanted.includes('/') || wanted.includes(path.sep)) {
		const candidate = path.resolve(wanted);
		return isRunnableFile(candidate) ? candidate : null;
	}

	const search = (env && env.PATH) || '';
	for (const entry of search.split(path.delimiter)) {
		if (entry === '') {
			continue;
		}
		const candidate = path.join(entry, wanted);
		if (isRunnableFile(candidate)) {
			return candidate;
		}
	}
	return null;
}

// reportMissing meldet das fehlende Programm und führt auf Wunsch direkt zur
// Einstellung. Als Meldung, nicht nur als Terminalausgabe: ob der Text eines
// mit Fehler endenden Programms im Terminal-Tab lesbar stehen bleibt, ist
// ungemessen — eine Meldung deckt beide Ausgänge ab.
async function reportMissing(executable, settingKey) {
	const key = settingKey || 'kPlaybook.openCode.executable';
	const open = 'Einstellung öffnen';
	const answer = await vscode.window.showErrorMessage(
		`Programm nicht gefunden: ${executable}. Erwartet wird es im PATH oder unter dem in „${key}“ eingestellten Pfad.`,
		open,
	);
	if (answer === open) {
		await vscode.commands.executeCommand('workbench.action.openSettings', key);
	}
}

module.exports = { resolveExecutable, reportMissing };
