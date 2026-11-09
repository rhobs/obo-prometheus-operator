#!/usr/bin/env bash
# Test doubles for external publication/build commands. Never contacts a service.
set -euo pipefail
name="${0##*/}"
jq -cn --arg name "$name" --arg work "${GOWORK:-}" --args \
	'{command:$name, args:$ARGS.positional, gowork:$work}' -- "$@" >> "$FAKE_LOG"
case "$name" in
docker)
	case "${1:-} ${2:-} ${3:-}" in
	"buildx imagetools inspect")
		[[ "${FAKE_REGISTRY_ERROR:-false}" == false ]] || { echo 'denied: authentication required' >&2; exit 1; }
		jq -e --arg ref "${!#}" 'has($ref)' "$FAKE_REGISTRY" >/dev/null ||
			{ echo 'manifest unknown' >&2; exit 1; };;
	"image inspect "*)
		jq -r --arg ref "${!#}" '.[$ref]' "$FAKE_REGISTRY";;
	"buildx imagetools create")
		while (($#)); do
			if [[ "$1" == --tag ]]; then
				jq --arg ref "$2" --arg commit "$FAKE_COMMIT" '.[$ref]=$commit' "$FAKE_REGISTRY" > "$FAKE_REGISTRY.tmp"
				mv -- "$FAKE_REGISTRY.tmp" "$FAKE_REGISTRY"
				break
			fi
			shift
		done;;
	esac;;
make)
	for arg do
		if [[ "$arg" == stripped-down-crds.yaml ]]; then printf 'kind: List\n' > stripped-down-crds.yaml; fi
	done;;
go) : ;; # Module/import tests assert the arguments rather than compiling operator code.
gh)
	if [[ "${1:-}" == api ]]; then
		printf '%s\n' "${FAKE_PRS:-[]}"
	else
		echo 'Unexpected attempt to create a real PR' >&2; exit 1
	fi;;
*) echo "Unexpected test command: $name" >&2; exit 1;;
esac
