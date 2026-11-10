#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

case "${1:-}" in
tools)
	env GOWORK=off GOFLAGS= go -C rhobs/release test -race -count=1 ./...
	env GOWORK=off GOFLAGS= go -C rhobs/release vet ./...
	[[ -z "$(gofmt -l rhobs/release)" ]]
	for script in rhobs/*.sh; do bash -n "$script"; done
	shellcheck rhobs/*.sh rhobs/olm/catalog/update-channels.sh
	;;
lint)
	make check-golang check-api check-license shellcheck check-metrics
	;;
unit)
	make test-unit
	;;
downstream)
	bash rhobs/release.sh verify
	bash rhobs/generate.sh --extras-only
	bash rhobs/release.sh check-import
	bash rhobs/release.sh assert-clean
	;;
*)
	echo "Usage: $0 {tools|lint|unit|downstream}" >&2
	exit 1
	;;
esac
