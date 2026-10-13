#!/usr/bin/env bash
# Packt defaults/ zum Release-Asset k-playbook-defaults.tar.gz.
#
# Kephalaion holt dieses Archiv aus dem neuesten Release und legt seinen Inhalt
# unter <hub>:k-playbook/defaults/ ab. Die Prüfungen hier sind dieselben, an
# denen der Hub ein Archiv ablehnt (internal/hub/source/fetch.go, Unpack): nur
# Dateien und Verzeichnisse, je Datei UTF-8 ohne NUL und höchstens 1 MiB, alle
# Inhalte zusammen höchstens 8 MiB. Scheitert eine davon, bricht der Lauf ab,
# bevor ein Asset entsteht, das der Hub nicht übernähme.
#
# VERSION kommt aus dem Wurzelverzeichnis, nicht aus defaults/: der
# Release-Ablauf zählt nur diese eine Datei hoch.
#
# Reproduzierbar: sortierte Namen, feste Besitzer, Rechte und Zeit (die des
# letzten Commits), gzip ohne Namen und Zeitstempel. Zweimal gepackt ergibt
# dieselbe Prüfsumme.
#
# Aufruf aus dem Wurzelverzeichnis: .github/scripts/pack-defaults.sh <ausgabe>

set -euo pipefail

out="${1:?Aufruf: pack-defaults.sh <ausgabe.tar.gz>}"
src="defaults"
max_file=$((1024 * 1024))
max_total=$((8 * 1024 * 1024))

fail() { echo "pack-defaults: $*" >&2; exit 1; }

test -d "$src" || fail "$src/ fehlt; aufrufen aus dem Wurzelverzeichnis des Repos."
test -f VERSION || fail "VERSION fehlt im Wurzelverzeichnis."
test ! -e "$src/VERSION" || fail "$src/VERSION darf es nicht geben; VERSION kommt beim Packen aus dem Wurzelverzeichnis."

other="$(find "$src" ! -type f ! -type d)"
test -z "$other" || fail "nur Dateien und Verzeichnisse erlaubt, gefunden:
$other"

total=0
while IFS= read -r -d '' file; do
	size="$(wc -c < "$file")"
	test "$size" -le "$max_file" || fail "$file ist größer als 1 MiB ($size Bytes)."
	iconv -f UTF-8 -t UTF-8 "$file" > /dev/null 2>&1 || fail "$file ist kein gültiges UTF-8."
	tr -d '\000' < "$file" | cmp -s - "$file" || fail "$file enthält ein NUL-Byte."
	total=$((total + size))
done < <(find "$src" -type f -print0)
total=$((total + $(wc -c < VERSION)))
test "$total" -le "$max_total" || fail "Inhalt zusammen $total Bytes, erlaubt sind $max_total."

stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
cp -R "$src"/. "$stage"/
cp VERSION "$stage"/VERSION

mtime="$(git log -1 --format=%ct)"
mkdir -p "$(dirname "$out")"
(
	cd "$stage"
	# Namen ohne führendes ./, damit die Einträge genau den Pfaden unter
	# defaults/ entsprechen.
	find . -mindepth 1 -maxdepth 1 -printf '%P\0' | sort -z |
		tar --null --files-from=- --sort=name \
			--owner=0 --group=0 --numeric-owner \
			--mode='u=rwX,go=rX' --mtime="@$mtime" \
			--format=ustar -cf -
) | gzip -n -9 > "$out"

count="$(tar -tzf "$out" | grep -cv '/$')"
echo "pack-defaults: $out — $count Dateien, $total Bytes Inhalt."
