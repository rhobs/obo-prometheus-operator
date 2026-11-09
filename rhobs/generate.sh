#!/usr/bin/env bash
# Shared by release preparation and CI. No version bump, commits, or publishing.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# The old vendor tree still refers to upstream self-modules while generating.
# Preserve the OpenShift dependency pins, then re-vendor the final RHOBS sources.
export GOFLAGS=-mod=mod
make tidy
case "${1:-}" in
"")
	# Same recipe as upstream checks.yaml; transformation excludes only doc checks.
	make --always-make format generate
	;;
--extras-only) ;;
*) echo "Usage: $0 [--extras-only]" >&2; exit 1 ;;
esac
make example/non-rbac/prometheus-operator.yaml stripped-down-crds.yaml
mkdir -p example/stripped-down-crds
mv stripped-down-crds.yaml example/stripped-down-crds/all.yaml
bash rhobs/release.sh reconcile-vendor
make -C rhobs/olm tools bundle IMAGE_REPO=quay.io/rhobs
