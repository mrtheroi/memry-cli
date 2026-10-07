#!/usr/bin/env bash
# Renders the Homebrew formula of a memry release to stdout:
#
#   scripts/update-formula.sh 1.0.0 > ../homebrew-tap/Formula/memry.rb
#
# It downloads the release's checksums.txt (or reads CHECKSUMS_FILE, when
# set) and fills packaging/homebrew/memry.rb.tmpl with the version and the
# sha256 of each darwin/linux, amd64/arm64 archive.
set -euo pipefail

if [ $# -ne 1 ] || [ -z "$1" ]; then
    echo "usage: $0 <version>" >&2
    exit 2
fi
version="${1#v}"
root="$(cd "$(dirname "$0")/.." && pwd)"

if [ -n "${CHECKSUMS_FILE:-}" ]; then
    checksums="$(cat "$CHECKSUMS_FILE")"
else
    checksums="$(curl -fsSL "https://github.com/mrtheroi/memry-cli/releases/download/v${version}/checksums.txt")"
fi

formula="$(cat "$root/packaging/homebrew/memry.rb.tmpl")"
formula="${formula//@VERSION@/$version}"
for target in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64; do
    archive="memry_${version}_${target}.tar.gz"
    sum="$(awk -v archive="$archive" '$2 == archive { print $1 }' <<<"$checksums")"
    if ! [[ "$sum" =~ ^[0-9a-f]{64}$ ]]; then
        echo "$0: no sha256 for $archive in checksums.txt" >&2
        exit 1
    fi
    placeholder="@SHA256_$(tr '[:lower:]' '[:upper:]' <<<"$target")@"
    formula="${formula//$placeholder/$sum}"
done

printf '%s\n' "$formula"
