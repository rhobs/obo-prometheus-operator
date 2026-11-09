#!/usr/bin/env bash
# Build under a commit-specific staging tag, then promote missing release tags.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
bash rhobs/release.sh verify

: "${IMAGE_ORG:?Set IMAGE_ORG to the release Quay namespace}"
CPU_ARCHS="${CPU_ARCHS:-amd64}"
version="v$(head -n1 VERSION)"
commit="$(git rev-parse HEAD)"
stage="sha-$commit"
images=(obo-prometheus-operator obo-prometheus-config-reloader obo-admission-webhook obo-prometheus-operator-bundle)
missing=()
error_file="$(mktemp)"
trap 'rm -f "$error_file"' EXIT

image_exists() {
	local ref="$1"
	if docker buildx imagetools inspect "$ref" > /dev/null 2> "$error_file"; then
		return 0
	elif grep -Eqi 'manifest unknown|no such manifest|404 Not Found' "$error_file" || grep -Fq "$ref: not found" "$error_file"; then
		return 1
	else
		cat "$error_file" >&2
		exit 1
	fi
}

verify_image() {
	local image="$1" ref="quay.io/$IMAGE_ORG/$1:$version"
	local arches="$CPU_ARCHS" arch revision
	[[ "$image" == obo-prometheus-operator-bundle ]] && arches=amd64
	# Reading config labels does not execute an image, even for foreign architectures.
	for arch in $arches; do
		docker pull --platform "linux/$arch" "$ref"
		revision="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "$ref")"
		[[ "$revision" == "$commit" ]] || {
			echo "Refusing to overwrite $ref: it belongs to a different or unknown commit" >&2
			exit 1
		}
	done
}

for image in "${images[@]}"; do
	if image_exists "quay.io/$IMAGE_ORG/$image:$version"; then
		verify_image "$image"
	else
		missing+=("$image")
	fi
done

((${#missing[@]})) || exit 0
REGISTRIES=quay.io IMAGE_ORG="$IMAGE_ORG" CPU_ARCHS="$CPU_ARCHS" \
	RELEASE_BUILD=true TAG="$stage" bash rhobs/push-container-images.sh
make -C rhobs/olm tools bundle-image bundle-push CONTAINER_RUNTIME=docker IMAGE_REPO="quay.io/$IMAGE_ORG" \
	BUNDLE_IMG="quay.io/$IMAGE_ORG/obo-prometheus-operator-bundle:$stage"

for image in "${missing[@]}"; do
	# Re-check after the build in case another publisher created this tag meanwhile.
	if image_exists "quay.io/$IMAGE_ORG/$image:$version"; then
		verify_image "$image"
	else
		docker buildx imagetools create --tag "quay.io/$IMAGE_ORG/$image:$version" "quay.io/$IMAGE_ORG/$image:$stage"
	fi
done
