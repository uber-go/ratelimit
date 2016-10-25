#!/bin/bash

set -e

COVER=cover
ROOT_PKG=go.uber.org/ratelimit

if [[ -d "$COVER" ]]; then
	rm -rf "$COVER"
fi
mkdir -p "$COVER"

i=0
for pkg in "$@"; do
	i=$((i + 1))

	extracoverpkg=""
	if [[ -f "$GOPATH/src/$pkg/.extra-coverpkg" ]]; then
		extracoverpkg=$( \
			sed -e "s|^|$pkg/|g" < "$GOPATH/src/$pkg/.extra-coverpkg" \
			| tr '\n' ',')
	fi

	coverpkg=$(go list -json "$pkg" | jq -r \
		--arg PKG "$pkg" \
		--arg ROOT "$ROOT_PKG" \
		--arg VENDOR "$ROOT_PKG/vendor" \
	'
		.Deps
		| map(select(startswith($ROOT)))
		| map(select(startswith($VENDOR) == false))
		| . + [$PKG]
		| join(",")
	')
	if [[ -n "$extracoverpkg" ]]; then
		coverpkg="$extracoverpkg$coverpkg"
	fi

	go test \
		-coverprofile "$COVER/cover.${i}.out" -coverpkg "$coverpkg" \
		-v "$pkg"
done

gocovmerge "$COVER"/*.out > cover.out
