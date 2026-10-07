#!/usr/bin/env bash
# Tests scripts/update-formula.sh against a local checksums.txt, without the
# network: run `scripts/update-formula_test.sh` from anywhere.
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/update-formula.sh"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

sha() { printf '%064d' "$1"; }
cat > "$work/checksums.txt" <<SUMS
$(sha 1)  memry_1.2.3_darwin_amd64.tar.gz
$(sha 2)  memry_1.2.3_darwin_arm64.tar.gz
$(sha 3)  memry_1.2.3_linux_amd64.tar.gz
$(sha 4)  memry_1.2.3_linux_arm64.tar.gz
$(sha 5)  memry_1.2.3_linux_arm64.tar.gz.sbom.json
SUMS

formula="$(CHECKSUMS_FILE="$work/checksums.txt" "$script" 1.2.3)" || fail "the script failed"

grep -q '@[A-Z0-9_]*@' <<<"$formula" && fail "placeholders left: $(grep -o '@[A-Z0-9_]*@' <<<"$formula" | sort -u | tr '\n' ' ')"
for pair in darwin_amd64:1 darwin_arm64:2 linux_amd64:3 linux_arm64:4; do
    target="${pair%%:*}"
    url="https://github.com/mrtheroi/memry-cli/releases/download/v1.2.3/memry_1.2.3_${target}.tar.gz"
    # The sha256 line follows its url line.
    grep -A1 -F "url \"$url\"" <<<"$formula" | grep -q -F "sha256 \"$(sha "${pair##*:}")\"" \
        || fail "no url and sha256 for $target"
done
grep -q 'depends_on "php"' <<<"$formula" && fail "the formula depends on php"
grep -q -F 'export MEMRY_EXECUTABLE="#{opt_bin}/memry"' <<<"$formula" || fail "no MEMRY_EXECUTABLE wrapper"

# A leading v is accepted, a missing archive and a missing version are errors.
prefixed="$(CHECKSUMS_FILE="$work/checksums.txt" "$script" v1.2.3)" || fail "v1.2.3 failed"
grep -q 'download/v1.2.3/memry_1.2.3_' <<<"$prefixed" || fail "v1.2.3 not accepted"
grep -v darwin_arm64 "$work/checksums.txt" > "$work/partial.txt"
CHECKSUMS_FILE="$work/partial.txt" "$script" 1.2.3 >/dev/null 2>&1 && fail "a missing archive did not fail"
"$script" >/dev/null 2>&1 && fail "a missing version did not fail"

echo "ok"
