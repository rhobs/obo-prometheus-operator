#!/usr/bin/env bash
# Release orchestration only; operator builds stay in the upstream Makefiles.
set -euo pipefail
shopt -s inherit_errexit
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ "${1:-}" == --root ]]; then cd -- "${2:?--root needs a directory}"; shift 2; fi
ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
# shellcheck source=rhobs/release-lib.sh
source "$SCRIPT_DIR/release-lib.sh"
SOURCE_REPOSITORY=https://github.com/openshift/prometheus-operator.git
DOWNSTREAM_REPOSITORY=https://github.com/rhobs/obo-prometheus-operator.git
if [[ -z "${TMPDIR:-}" && -d /tmp/opencode ]]; then export TMPDIR=/tmp/opencode; fi
WORK="$(mktemp -d "${TMPDIR:-/tmp}/rhobs-release.XXXXXX")"
trap 'rm -rf -- "$WORK"' EXIT

plan_release() {
	local previous_commit previous_data="" branch base scripts version tag latest changed=true
	local previous_source="" previous_base="" source_tip
	load_tags "$downstream"
	[[ -s "$WORK/versions" ]] || die "A previously published downstream release is required"
	previous="${previous#v}"
	previous="${previous:-$(tail -n1 "$WORK/versions")}"
	valid_version "$previous"
	has_tag "v$previous" || die "Previous release v$previous does not exist"
	previous_commit=$(fetch_ref "$downstream" "refs/tags/v$previous" "refs/rhobs/releases/$previous")
	if git cat-file -e "$previous_commit:rhobs/release.json" 2>/dev/null; then
		previous_data="$WORK/previous.json"
		git show "$previous_commit:rhobs/release.json" > "$previous_data"
		validate_metadata "$previous_data"
		[[ "$(meta .version "$previous_data")" == "$previous" ]] || die "Previous tag and metadata disagree"
		while IFS= read -r tag; do has_tag "$tag" || die "Previous release has partial tag publication"; done < <(release_tags "$previous")
		previous_source=$(meta .source_commit "$previous_data")
		previous_base=$(meta .base_version "$previous_data")
	fi
	if [[ "$mode" == rebuild ]]; then
		[[ -n "$previous_data" ]] || die "Legacy releases lack provenance; start a new series with --mode new"
		[[ -z "$source" || "$source" == "$(meta .source_repository "$previous_data")" ]] ||
			die "A rebuild cannot change its source repository"
		source=$(meta .source_repository "$previous_data")
		branch=$(meta .source_branch "$previous_data")
		latest=$(grep -E "^${previous_base//./\\.}-rhobs[1-9][0-9]*$" "$WORK/versions" | tail -n1)
		[[ "$previous" == "$latest" ]] || die "Rebuild from the latest published revision: $latest"
	else
		source="${source:-$SOURCE_REPOSITORY}"
		git ls-remote --heads "$source" > "$WORK/heads"
		branch=$(awk '{sub(/^refs\/heads\//,"",$2); print $2}' "$WORK/heads" |
			{ grep -E "^$BRANCH_RE$" || true; } | LC_ALL=C sort -V | tail -n1)
		[[ -n "$branch" ]] || die "No release-x.y branches found"
	fi
	source_tip=$(fetch_ref "$source" "refs/heads/$branch" "refs/rhobs/source/$branch")
	if [[ -n "$source_commit" ]]; then
		[[ "$source_commit" =~ ^[0-9a-f]{40}$ ]] || die "--source-commit must be a full lowercase SHA"
		git merge-base --is-ancestor "$source_commit" "$source_tip" || die "Requested source commit is not on the selected branch"
	else source_commit="$source_tip"; fi
	base=$(git show "$source_commit:VERSION")
	[[ "$base" =~ ^$BASE_RE$ ]] || die "Source VERSION is not stable: $base"
	if [[ "$mode" == rebuild ]]; then
		[[ "$base" == "$previous_base" ]] || die "Source VERSION changed; explicitly start a new series"
		git merge-base --is-ancestor "$previous_source" "$source_commit" ||
			die "Source history was rewritten or this snapshot is older than the last release"
		[[ "$source_commit" != "$previous_source" || "$force" == true ]] || changed=false
	fi
	scripts=$(git rev-parse "$scripts_ref^{commit}")
	git cat-file -e "$scripts:rhobs/make-release-commit.sh"
	version=$(next_version "$base")
	[[ "$version" != "$previous" && "$(printf '%s\n' "$previous" "$version" | LC_ALL=C sort -V | tail -n1)" == "$version" ]] ||
		die "The selected source would downgrade the previous downstream release"
	jq -n --arg repo "$source" --arg branch "$branch" --arg commit "$source_commit" --arg base "$base" \
		--arg version "$version" --arg previous "$previous" --arg old "$previous_source" --arg scripts "$scripts" \
		--argjson changed "$changed" '{schema_version:1, source_repository:$repo, source_branch:$branch,
		source_commit:$commit, source_version:$base, base_version:$base, version:$version,
		previous_version:$previous, previous_source_commit:(if $old == "" then null else $old end),
		scripts_commit:$scripts, changed:$changed}'
}

prepare_release() {
	local version commit scripts target branch repository="" fork="" url existing base
	cat "$PLAN"
	if [[ "$(meta .changed)" == false ]]; then echo "Source is unchanged; no rebuild needed" >&2; return; fi
	if [[ "$execute" == false ]]; then echo "Dry run: use --execute to prepare a local release checkout" >&2; return; fi
	assert_clean
	version=$(meta .version); commit=$(meta .source_commit); scripts=$(meta .scripts_commit)
	target="rhobs-rel-$version"; branch="rhobs-pr-$version"
	if [[ "$publish_pr" == true ]]; then
		[[ -n "$fork_repository" ]] || die "--publish-pr requires --fork-repository owner/repo"
		repository=$(github_repository "$downstream"); fork=$(github_repository "$fork_repository")
		gh api --method GET "repos/$repository/pulls" -f state=open -f "base=$target" \
			-f "head=${fork%%/*}:$branch" > "$WORK/prs.json"
		url=$(jq -r '.[0].html_url // empty' "$WORK/prs.json")
		if [[ -n "$url" ]]; then echo "$url"; return; fi
	fi
	worktree="${worktree:-$(mktemp -d "${TMPDIR:-/tmp}/rhobs-release-worktree.XXXXXX")}"
	worktree=$(realpath -m -- "$worktree")
	git worktree add -b "$branch" "$worktree" "$commit"
	echo "Release checkout: $worktree" >&2
	(cd "$worktree"; apply_overlay "$scripts")
	git -C "$worktree" commit -s -m "git: apply rhobs-scripts overlay"
	local -a checks=()
	[[ "$no_checks" == false ]] || checks=(--no-checks)
	(cd "$worktree"; bash rhobs/make-release-commit.sh --plan "$PLAN" "${checks[@]}")
	if [[ "$publish_pr" == false ]]; then echo "Prepared $branch; nothing has been pushed" >&2; return; fi
	existing=$(git ls-remote --heads "$downstream" "refs/heads/$target")
	base="${existing%%$'\t'*}"
	[[ -z "$existing" || "$base" == "$commit" ]] || die "$target already exists at a different commit"
	git -C "$worktree" push "$downstream" "$commit:refs/heads/$target"
	git -C "$worktree" push "https://github.com/$fork.git" "HEAD:refs/heads/$branch"
	{
		# shellcheck disable=SC2016 # Markdown code spans are literal backticks.
		printf 'Source: %s (%s)\n\nSource commit: `%s`\n\nScripts commit: `%s`\n\nUpgrade baseline: `v%s`\n' \
			"$(meta .source_repository)" "$(meta .source_branch)" "$commit" "$scripts" "$(meta .previous_version)"
		local old source_url
		old=$(meta '.previous_source_commit // empty'); source_url=$(meta .source_repository)
		if [[ -n "$old" ]]; then printf '\nSource changes: %s/compare/%s...%s\n' "${source_url%.git}" "$old" "$commit"; fi
	} > "$WORK/pr-body"
	gh pr create --repo "$repository" --base "$target" --head "${fork%%/*}:$branch" \
		--title "chore(release): v$version" --body-file "$WORK/pr-body"
}

publish_tags() {
	verify_release
	assert_clean
	local version head tag existing
	local -a refs=()
	version=$(meta .version); head=$(git rev-parse HEAD)
	load_tags "$downstream"
	while IFS= read -r tag; do
		if has_tag "$tag"; then
			existing=$(fetch_ref "$downstream" "refs/tags/$tag" "refs/rhobs/published/$tag")
			[[ "$existing" == "$head" ]] || die "Published tag $tag points at another commit; refusing to overwrite"
			refs+=("refs/rhobs/published/$tag:refs/tags/$tag")
		else
			if git show-ref --verify --quiet "refs/tags/$tag"; then
				[[ "$(git rev-parse "refs/tags/$tag^{commit}")" == "$head" ]] || die "Local tag $tag points at another commit"
			else git tag -a "$tag" -m "v$version" "$head"; fi
			refs+=("refs/tags/$tag:refs/tags/$tag")
		fi
	done < <(release_tags "$version")
	git push --atomic "$downstream" "${refs[@]}"
}

check_import() {
	cp -- rhobs/test/import/go.mod rhobs/test/import/go.sum "$WORK/"
	local modfile="$WORK/go.mod" module path
	for module in "$MODULE" "$MODULE/pkg/apis/monitoring" "$MODULE/pkg/client"; do
		if [[ "$published" == true ]]; then
			(cd rhobs/test/import; GOWORK=off go mod edit "-modfile=$modfile" -dropreplace "$module")
		else
			path="$ROOT${module#"$MODULE"}"
			(cd rhobs/test/import; GOWORK=off go mod edit "-modfile=$modfile" -replace "$module=$path")
		fi
	done
	(cd rhobs/test/import; GOWORK=off go test -mod=mod "-modfile=$modfile" ./...)
}

command="${1:---help}"; (($# == 0)) || shift
mode="" source="" previous="" source_commit="" scripts_ref=HEAD force=false PLAN="" worktree=""
downstream="$DOWNSTREAM_REPOSITORY" execute=false no_checks=false publish_pr=false fork_repository="" published=false ref=""
while (($#)); do
	case "$1" in
	--mode|--source|--downstream|--previous-version|--source-commit|--scripts-ref|--plan|--worktree|--fork-repository|--remote|--ref)
		(($# >= 2)) && [[ -n "$2" ]] || die "$1 requires a value"
		case "$1" in
		--mode) mode="$2";; --source) source="$2";; --downstream|--remote) downstream="$2";;
		--previous-version) previous="$2";; --source-commit) source_commit="$2";; --scripts-ref) scripts_ref="$2";;
		--plan) PLAN="$2";; --worktree) worktree="$2";; --fork-repository) fork_repository="$2";; --ref) ref="$2";;
		esac
		shift 2;;
	--force) force=true; shift;; --execute) execute=true; shift;; --no-checks) no_checks=true; shift;;
	--publish-pr) publish_pr=true; shift;; --published) published=true; shift;;
	*) die "Unknown option: $1";;
	esac
done
case "$command" in
plan|prepare)
	[[ "$publish_pr" == false || "$execute" == true ]] || die "--publish-pr requires --execute"
	if [[ -n "$PLAN" ]]; then
		[[ "$command" == prepare && -z "$mode$source$source_commit$previous" && "$force" == false && "$scripts_ref" == HEAD ]] ||
			die "--plan cannot be combined with source/version selection arguments"
		PLAN=$(realpath -- "$PLAN"); validate_metadata "$PLAN"
		jq -e '(.changed | type) == "boolean"' "$PLAN" >/dev/null || die "Plan requires a boolean changed field"
		if [[ "$(meta .changed)" == true ]]; then
			load_tags "$downstream"
			[[ "$(next_version "$(meta .base_version)")" == "$(meta .version)" ]] || die "Release plan is stale"
		fi
	else
		[[ "$mode" == new || "$mode" == rebuild ]] || die "--mode must be new or rebuild"
		PLAN="$WORK/plan.json"
		plan_release > "$PLAN"
	fi
	if [[ "$command" == plan ]]; then cat "$PLAN"; else prepare_release; fi;;
transform|go-version)
	[[ -n "$PLAN" ]] || die "--plan is required"
	PLAN=$(realpath -- "$PLAN"); validate_metadata "$PLAN"
	if [[ "$command" == transform ]]; then
		# shellcheck source=rhobs/transform-release.sh
		source "$SCRIPT_DIR/transform-release.sh"
	else
		git show "$(meta .source_commit):go.mod" > "$WORK/source.mod"
		awk '$1 == "go" && $2 ~ /^[0-9]+\.[0-9]+(\.[0-9]+)?$/ {print $2; found=1; exit} END {exit !found}' "$WORK/source.mod"
	fi;;
verify) verify_release; cat "$PLAN";;
assert-clean) assert_clean;;
check-vendor|reconcile-vendor)
	PLAN=rhobs/release.json; validate_metadata "$PLAN"
	if [[ "$command" == reconcile-vendor ]] && git cat-file -e "$(meta .source_commit):vendor" 2>/dev/null; then GOWORK=off go mod vendor; fi
	check_vendor;;
publish-tags)
	[[ "$downstream" != "$DOWNSTREAM_REPOSITORY" ]] || downstream=origin
	publish_tags;;
check-import) check_import;;
sync-ci)
	[[ -n "$ref" ]] || die "--ref is required"
	ref=$(git rev-parse --verify "$ref^{commit}")
	commit=$(sync_ci "$ref"); echo "Restored upstream CI from $commit";;
-h|--help)
	echo "Usage: $0 [--root REPOSITORY] COMMAND [OPTIONS]"
	echo "Commands: plan prepare transform verify assert-clean check-vendor reconcile-vendor publish-tags check-import go-version sync-ci"
	echo "Use plan --mode {new|rebuild}; prepare --plan FILE [--execute [--publish-pr --fork-repository OWNER/REPO]]";;
*) die "Unknown command: $command";;
esac
