#!/usr/bin/env bash
# Keep the standalone Go module independent of the operator's modules/vendor.
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ -n "${RHOBS_RELEASE_BIN:-}" ]]; then
	exec "$RHOBS_RELEASE_BIN" --root "$root" "$@"
fi
exec env GOWORK=off GOFLAGS= go -C "$script_dir/release" run . --root "$root" "$@"
