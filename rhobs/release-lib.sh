#!/usr/bin/env bash
# Shared Git/metadata helpers. Sourced by release.sh with strict Bash settings.

MODULE=github.com/rhobs/obo-prometheus-operator
UPSTREAM_MODULE=github.com/prometheus-operator/prometheus-operator
BASE_RE='(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)'
BRANCH_RE='release-(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)'
VERSION_RE="$BASE_RE-rhobs[1-9][0-9]*"
CI_FILES=(checks.yaml unit.yaml changed-files.yaml e2e.yaml e2e-feature-gated.yaml
	test-prom-version-upgrade.yaml e2e-prometheus-v2.yaml actionlint.yml)

die() { echo "Release error: $*" >&2; exit 1; }
meta() { jq -r "$1" "${2:-$PLAN}"; }
assert_clean() { [[ -z "$(git status --porcelain --untracked-files=all)" ]] || die "Checkout is not clean"; }
valid_version() { [[ "$1" =~ ^$VERSION_RE$ ]] || die "Invalid downstream version: $1"; }
release_tags() { printf '%s\n' "v$1" "pkg/apis/monitoring/v$1" "pkg/client/v$1"; }

validate_metadata() {
	jq -e --arg base "^$BASE_RE$" --arg version "^$VERSION_RE$" --arg branch "^$BRANCH_RE$" '
		type == "object" and .schema_version == 1
		and ([.source_repository, .source_branch, .source_commit, .source_version,
			.base_version, .version, .previous_version, .scripts_commit]
			| all(type == "string" and length > 0))
		and (.source_branch | test($branch))
		and ([.source_commit, .scripts_commit] | all(test("^[0-9a-f]{40}$")))
		and (.ci_commit == null or .ci_commit == "" or (.ci_commit | test("^[0-9a-f]{40}$")))
		and (.base_version | test($base)) and .source_version == .base_version
		and (.version | test($version)) and (.previous_version | test($version))
		and (.version | split("-rhobs")[0]) == .base_version
	' "$1" >/dev/null || die "Missing or invalid release metadata: $1"
}

fetch_ref() {
	git fetch --no-tags "$1" "+$2:$3" >&2
	git rev-parse "$3^{commit}"
}

load_tags() {
	git ls-remote --tags "$1" > "$WORK/tags"
	awk '$2 ~ /^refs\/tags\/v/ {sub(/^refs\/tags\/v/, "", $2); print $2}' "$WORK/tags" |
		{ grep -E "^$VERSION_RE$" || true; } | LC_ALL=C sort -V > "$WORK/versions"
}

has_tag() { awk -v ref="refs/tags/$1" '$2 == ref {found=1} END {exit !found}' "$WORK/tags"; }

# Increment decimal text, not a machine integer (rhobs9 -> rhobs10 without overflow).
increment() {
	local number="$1" zeros="" digit i
	for ((i=${#number}-1; i>=0; i--)); do
		digit="${number:i:1}"
		if [[ "$digit" != 9 ]]; then
			printf '%s%s%s\n' "${number:0:i}" "$((digit + 1))" "$zeros"
			return
		fi
		zeros="0$zeros"
	done
	printf '1%s\n' "$zeros"
}

next_version() {
	local base="$1" last revision=0 tag version
	[[ "$base" =~ ^$BASE_RE$ ]] || die "Invalid source VERSION: $base"
	last=$(grep -E "^${base//./\\.}-rhobs[1-9][0-9]*$" "$WORK/versions" | tail -n1 || true)
	[[ -z "$last" ]] || revision="${last##*-rhobs}"
	version="$base-rhobs$(increment "$revision")"
	while IFS= read -r tag; do
		if has_tag "$tag"; then die "Partial publication exists for $version; recover it first"; fi
	done < <(release_tags "$version")
	printf '%s\n' "$version"
}

github_repository() {
	local repo="${1#https://github.com/}"
	repo="${repo#git@github.com:}"
	repo="${repo%.git}"
	[[ "$repo" =~ ^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$ ]] || die "Expected GitHub owner/repo: $1"
	printf '%s\n' "$repo"
}

apply_overlay() {
	local -a paths
	git ls-tree -rz --name-only "$1" > "$WORK/overlay-paths"
	mapfile -d '' -t paths < "$WORK/overlay-paths"
	git restore --source="$1" --staged --worktree -- "${paths[@]}"
}

ci_commit() {
	local source="$1" commit candidate
	if git cat-file -e "$source:.github/workflows/checks.yaml" 2>/dev/null; then echo "$source"; return; fi
	git log --first-parent --format=%H "$source" -- .github/workflows/checks.yaml > "$WORK/ci-history"
	while IFS= read -r commit; do
		for candidate in "$commit" "$commit^"; do
			if git cat-file -e "$candidate:.github/workflows/checks.yaml" 2>/dev/null; then
				git rev-parse "$candidate^{commit}"; return
			fi
		done
	done < "$WORK/ci-history"
	die "No upstream CI found; fetch complete source history"
}

sync_ci() {
	local source="$1" commit directory file action=.github/actions/setup-e2e/action.yaml
	commit=$(ci_commit "$source")
	directory=$(mktemp -d "$WORK/ci.XXXXXX")
	mkdir -p "$directory/.github/workflows"
	for file in "${CI_FILES[@]}"; do
		git show "$commit:.github/workflows/$file" > "$directory/.github/workflows/$file"
	done
	git -C "$directory" apply --check "$ROOT/rhobs/ci.patch" ||
		die "Upstream CI changed; refresh rhobs/ci.patch"
	git -C "$directory" apply "$ROOT/rhobs/ci.patch"
	if git cat-file -e "$source:$action" 2>/dev/null; then
		git show "$source:$action" > "$directory/action.yaml"
	else
		git show "$commit:$action" > "$directory/action.yaml"
	fi
	# Validate everything before replacing any destination files.
	mkdir -p .github/workflows .github/actions/setup-e2e
	for file in "${CI_FILES[@]}"; do
		{
			printf '# Generated from openshift/prometheus-operator@%s.\n' "$commit"
			echo '# Downstream changes: rhobs/ci.patch; refresh with rhobs/release.sh sync-ci.'
			echo
			cat "$directory/.github/workflows/$file"
		} > ".github/workflows/$file"
	done
	cp -- "$directory/action.yaml" "$action"
	echo "$commit"
}

first_party_vendor() {
	[[ "$1" == "vendor/$MODULE/"* || "$1" == "vendor/$UPSTREAM_MODULE/"* ]]
}

check_source_vendor() {
	local commit="$1" entry path original hash
	local -A blobs=()
	git ls-tree -rz "$commit" -- pkg "vendor/$MODULE" "vendor/$UPSTREAM_MODULE" > "$WORK/self-vendor"
	while IFS=$'\t' read -r -d '' entry path; do blobs["$path"]="${entry##* }"; done < "$WORK/self-vendor"
	for path in "${!blobs[@]}"; do
		if ! first_party_vendor "$path"; then continue; fi
		original="${path#vendor/"$UPSTREAM_MODULE"/}"
		original="${original#vendor/"$MODULE"/}"
		hash="${blobs[$original]:-}"
		if [[ "$hash" == "${blobs[$path]}" ]]; then continue; fi
		if [[ -z "$hash" && "${original##*/}" =~ ^(LICENSE|NOTICE|COPYING)$ ]]; then continue; fi
		die "Source has a self-module vendor-only patch at $path; reconcile it with $original"
	done
}

vendor_manifest() {
	local entry path
	while IFS=$'\t' read -r -d '' entry path; do
		if [[ "$path" == vendor/modules.txt ]] || first_party_vendor "$path"; then continue; fi
		printf '%s\t%s\0' "$path" "${entry##* }"
	done | LC_ALL=C sort -z
}

check_vendor() {
	local source path hash
	source=$(meta .source_commit)
	git ls-tree -rz "$source" -- vendor > "$WORK/vendor-tree"
	vendor_manifest < "$WORK/vendor-tree" > "$WORK/vendor-expected"
	: > "$WORK/vendor-paths"
	if [[ -d vendor ]]; then
		find vendor \( -type f -o -type l \) -print0 > "$WORK/vendor-paths"
	fi
	# Compare raw bytes, including ignored/new files and symlink targets. Never
	# stage vendor files or let Git clean filters hide a changed OpenShift patch.
	while IFS= read -r -d '' path; do
		if [[ "$path" == vendor/modules.txt ]] || first_party_vendor "$path"; then continue; fi
		if [[ -L "$path" ]]; then hash=$(readlink -n -- "$path" | git hash-object --stdin)
		else hash=$(git hash-object --no-filters -- "$path"); fi
		printf '%s\t%s\0' "$path" "$hash"
	done < "$WORK/vendor-paths" | LC_ALL=C sort -z > "$WORK/vendor-actual"
	cmp -s "$WORK/vendor-expected" "$WORK/vendor-actual" ||
		die "Regeneration changed OpenShift third-party vendor files; preserve or reconcile these fixes"
}

verify_release() {
	PLAN=rhobs/release.json
	validate_metadata "$PLAN"
	local source version branch
	source=$(meta .source_commit); version=$(meta .version)
	[[ "$(head -n1 VERSION)" == "$version" ]] || die "VERSION and release metadata disagree"
	[[ "$(git show "$source:VERSION")" == "$(meta .source_version)" ]] || die "Pinned source VERSION disagrees"
	git merge-base --is-ancestor "$source" HEAD || die "Source commit is not an ancestor of this release"
	branch=$(git branch --show-current)
	[[ "$branch" != rhobs-rel-* || "$branch" == "rhobs-rel-$version" ]] || die "Release branch and VERSION disagree"
	[[ "${GITHUB_REF:-}" != refs/heads/rhobs-rel-* || "$GITHUB_REF" == "refs/heads/rhobs-rel-$version" ]] ||
		die "GitHub release branch and VERSION disagree"
	check_vendor
}
