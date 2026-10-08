#!/bin/sh
# README animations, recorded with the termproof CLI (go tool termproof) and
# embedded into README.md between marker comments:
#
#   <!-- asset:footer-layout -->
#   ![footer-layout](docs/assets/footer-layout.svg)
#   <!-- /asset:footer-layout -->
#
#   sh scripts/assets.sh list            # scene names
#   sh scripts/assets.sh embed           # only rewrite the README embeds
#   sh scripts/assets.sh [scene ...]     # record all scenes (or the named ones), then embed
#
# One line per scene below: <name> <example flags...>. Output: docs/assets/<name>.svg
set -eu
cd "$(dirname "$0")/.."

scenes() {
  cat <<'TABLE'
footer-layout
row-layout -layout=row
parallel-rows -mode=parallel
scattered -mode=scattered
beats -mode=beats
TABLE
}
names() { scenes | cut -d' ' -f1; }

# embed rewrites the line(s) between each scene's markers in README.md.
embed() {
  tmp=$(mktemp)
  awk -v names="$(names | tr '\n' ' ')" '
    BEGIN { n = split(names, arr, " "); for (i = 1; i <= n; i++) known[arr[i]] = 1 }
    /^<!-- asset:[a-z0-9-]+ -->$/ {
      name = $0; sub(/^<!-- asset:/, "", name); sub(/ -->$/, "", name)
      if (!(name in known)) { print "unknown scene in README marker: " name > "/dev/stderr"; exit 2 }
      print; print "![" name "](docs/assets/" name ".svg)"; seen[name] = 1; skipping = 1; next
    }
    /^<!-- \/asset:[a-z0-9-]+ -->$/ { skipping = 0 }
    !skipping { print }
    END { for (i = 1; i <= n; i++) if (!(arr[i] in seen)) { print "README has no <!-- asset:" arr[i] " --> marker" > "/dev/stderr"; exit 2 } }
  ' README.md > "$tmp"
  mv "$tmp" README.md
}

case "${1:-}" in
  list) names; exit 0 ;;
  embed) embed; exit 0 ;;
esac

for want in "$@"; do
  names | grep -qx "$want" || { echo "unknown scene: $want (see: list)" >&2; exit 2; }
done

bin=$(mktemp -t simple-loader.XXXXXX)
trap 'rm -f "$bin"' EXIT
CGO_ENABLED=0 go build -buildvcs=false -o "$bin" ./examples/simple-loader
mkdir -p docs/assets

scenes | while read -r name flags; do
  if [ $# -gt 0 ]; then
    case " $* " in *" $name "*) ;; *) continue ;; esac
  fi
  # shellcheck disable=SC2086
  CGO_ENABLED=0 go tool termproof record -o "docs/assets/$name.svg" -cols 80 -rows 16 -- "$bin" $flags \
    || true  # the example exits 1 when it injects errors; that is part of the demo
done

embed
