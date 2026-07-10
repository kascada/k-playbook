#!/usr/bin/env bash
# diff-overlay.sh – Overlay-Repo gegen Base diffen
#
# Usage: ./diff-overlay.sh <base-pfad> <overlay-pfad> [subpfad]
#
# Beispiele:
#   ./diff-overlay.sh cloudetair-chatbot-api squad-cdt-chatbot-api
#     → diff -rq beider app/-Verzeichnisse + Zeilenstatistik pro geänderter Datei
#
#   ./diff-overlay.sh _bases/base-xyz/app squad-repo app/logic
#     → diff nur im Unterpfad app/logic

set -uo pipefail   # WICHTIG: KEIN -e, weil `diff` Exit 1 bei Unterschieden liefert

BASE="${1:?Usage: $0 <base-path> <overlay-path> [subpath]}"
OVR="${2:?Usage: $0 <base-path> <overlay-path> [subpath]}"
SUB="${3:-app}"

BASE_DIR="$BASE/$SUB"
OVR_DIR="$OVR/$SUB"

if [[ ! -d "$BASE_DIR" ]]; then
  echo "✗ Base-Pfad existiert nicht: $BASE_DIR" >&2
  exit 1
fi
if [[ ! -d "$OVR_DIR" ]]; then
  echo "✗ Overlay-Pfad existiert nicht: $OVR_DIR" >&2
  exit 1
fi

echo "===================================================================="
echo "Base:    $BASE_DIR"
echo "Overlay: $OVR_DIR"
echo "===================================================================="
echo ""

# Zwischenspeichern, weil wir mehrfach parsen
DIFF_OUT=$(diff -rq "$BASE_DIR" "$OVR_DIR" 2>/dev/null)

echo "=== Datei-Liste (was unterscheidet sich?) ==="
echo "$DIFF_OUT"
echo ""

echo "=== Zeilenstatistik pro geänderter Datei ==="
CHANGED_COUNT=$(echo "$DIFF_OUT" | grep -c "^Files " || true)
if [[ "$CHANGED_COUNT" -eq 0 ]]; then
  echo "  (keine überschriebenen Dateien)"
else
  echo "$DIFF_OUT" | grep "^Files " | while IFS= read -r line; do
    f_base=$(echo "$line" | awk '{print $2}')
    f_ovr=$(echo "$line"  | awk '{print $4}')
    rel_path=${f_ovr#"$OVR_DIR/"}

    # WICHTIG: || true, weil diff bei Unterschieden Exit 1 liefert
    stats=$(diff "$f_base" "$f_ovr" 2>/dev/null \
      | awk '/^</{d++} /^>/{a++} END{printf "%4d gel., %4d hinz., netto %+5d", d+0, a+0, (a+0)-(d+0)}' \
      || true)

    printf "  %-60s  %s\n" "$rel_path" "$stats"
  done
fi
echo ""

echo "=== Nur im Overlay (neu, nicht in Base) ==="
NEW_IN_OVR=$(echo "$DIFF_OUT" | grep "^Only in $OVR_DIR" || true)
if [[ -z "$NEW_IN_OVR" ]]; then
  echo "  (keine)"
else
  echo "$NEW_IN_OVR"
fi
echo ""

echo "=== Nur in Base (im Overlay nicht überschrieben – normal für Overlay-Pattern) ==="
ONLY_BASE_COUNT=$(echo "$DIFF_OUT" | grep -c "^Only in $BASE_DIR" || true)
echo "  Anzahl: $ONLY_BASE_COUNT Einträge"
echo ""

echo "Tipp: Für konkrete Diffs:"
echo "  diff -u $BASE_DIR/<datei> $OVR_DIR/<datei> | less"
