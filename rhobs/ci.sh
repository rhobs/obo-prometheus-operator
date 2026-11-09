#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

case "${1:-}" in
tools)
	bash rhobs/test/release_test.sh
	for script in rhobs/*.sh rhobs/test/*.sh; do bash -n "$script"; done
	shellcheck rhobs/*.sh rhobs/test/*.sh rhobs/olm/catalog/update-channels.sh
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
