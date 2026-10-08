#!/bin/sh
# docs:check: every docs/assets/*.svg is referenced by README.md, every README
# image path exists, every recorder scene has a file and a README marker, and
# the embeds between markers are what `scripts/assets.sh embed` would write.
set -eu
cd "$(dirname "$0")/.."
fail=0
for f in docs/assets/*.svg; do
  grep -q "($f)" README.md || { echo "unreferenced asset: $f"; fail=1; }
done
for p in $(grep -o '](docs/assets/[^)]*)' README.md | sed 's/^](//; s/)$//' | sort -u); do
  [ -f "$p" ] || { echo "README references missing file: $p"; fail=1; }
done
scenes=$(sh scripts/assets.sh list) || { echo "could not list scenes"; exit 1; }
[ -n "$scenes" ] || { echo "recorder listed no scenes"; exit 1; }
for s in $scenes; do
  [ -f "docs/assets/$s.svg" ] || { echo "scene without asset: $s (run: task assets -- $s)"; fail=1; }
  grep -qx "<!-- asset:$s -->" README.md || { echo "README has no <!-- asset:$s --> marker"; fail=1; }
done
tmp=$(mktemp); cp README.md "$tmp"
if sh scripts/assets.sh embed && [ "$(cat README.md)" != "$(cat "$tmp")" ]; then
  echo "README embeds are out of date (run: task assets:embed)"; fail=1
fi
cp "$tmp" README.md; rm -f "$tmp"
exit $fail
