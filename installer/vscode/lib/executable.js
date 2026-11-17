// Auflösung eines Programms zu seinem absoluten Pfad.
//
// Aufgelöst wird im Extension-Host, der den PATH der Login-Shell trägt (unter
// WSL einschließlich `~/.local/bin`). So fällt ein fehlendes Programm auf,
// bevor es gestartet wird, und der Aufruf hängt nicht an der Umgebung des
// gestarteten Prozesses.
'use strict';

const fs = require('node:fs');
const path = require('node:path');

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

module.exports = { resolveExecutable };
