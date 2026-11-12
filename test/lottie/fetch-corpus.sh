#!/usr/bin/env bash
#
# fetch-corpus.sh downloads the Noto animated emoji as Lottie JSON into
# testdata/emoji, for the tests of this folder (D41). The emoji are published
# by Google under CC BY 4.0. The script writes a NOTICE next to them.

set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"
out=testdata/emoji
mkdir -p "$out"
api=https://googlefonts.github.io/noto-emoji-animation/data/api.json
host=https://fonts.gstatic.com/s/e/notoemoji/latest

curl -fsSL --retry 3 "$api" |
  python3 -I -c '
import json, sys
for icon in json.load(sys.stdin).get("icons", []):
    cp = icon.get("codepoint")
    if cp:
        print(cp)
' |
  while read -r cp; do
    dest="$out/$cp.json"
    [ -s "$dest" ] && continue
    if curl -fsSL --retry 3 -o "$dest.tmp" "$host/$cp/lottie.json"; then
      mv "$dest.tmp" "$dest"
    else
      rm -f "$dest.tmp"
    fi
  done

cat > "$out/NOTICE" <<NOTICE
Noto animated emoji, downloaded from $api.
Copyright Google Inc. Licensed under CC BY 4.0:
https://creativecommons.org/licenses/by/4.0/
NOTICE
ls "$out"/*.json | wc -l
