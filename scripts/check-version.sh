#!/usr/bin/env sh
# Usage: scripts/check-version.sh 0.1.0
# Fails unless every place that names the release version agrees with it.
set -eu

version="${1:?usage: $0 <version>}"
root="$(cd "$(dirname "$0")/.." && pwd)"
chart="$root/charts/k-watch/Chart.yaml"
status=0

check() {
  if [ "$2" != "$version" ]; then
    echo "$1 is '$2', expected '$version'" >&2
    status=1
  fi
}

check "Chart.yaml version" "$(sed -n 's/^version: *//p' "$chart" | tr -d '"')"
check "Chart.yaml appVersion" "$(sed -n 's/^appVersion: *//p' "$chart" | tr -d '"')"
check "Chart.yaml images annotation" "$(sed -n 's/.*image: *ghcr.io\/klolarion\/k-watch://p' "$chart")"
for name in README.md README.ko.md charts/k-watch/README.md; do
  for found in $(grep -o -- '--version [0-9][^ ]*' "$root/$name" | cut -d' ' -f2 | sort -u); do
    check "$name --version" "$found"
  done
done

[ "$status" -eq 0 ] && echo "version $version is consistent"
exit "$status"
