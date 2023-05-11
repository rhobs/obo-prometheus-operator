#!/usr/bin/env bash
set -e -u -o pipefail

PROJECT_ROOT="$(git rev-parse --show-toplevel)"
declare -r PROJECT_ROOT
declare -r IMG_REPO="quay.io/rhobs"
declare -r IMG_PREFIX="obo-"

# config vars
declare SHOW_USAGE=false
declare IGNORE_REPO_STATE=false
declare RUN_MAKE_CHECKS=true
declare PREVIOUS_VERSION=""

# NOTE: this var is computed in `bumpup_version`
declare RHOBS_VERSION=""

header() {
	echo -e "\n 🔆 $*"
	echo -e "─────────────────────────────────────────────────────\n"
}

info() {
	echo " 🔔 $*"
}

ok() {
	echo " ✔  $*"
}

warn() {
	echo " ⚠️  $*"
}

die() {
	echo -e "\n ✋ $* "
	echo -e "──────────────────── ⛔️⛔️⛔️ ────────────────────────\n"
	exit 1
}

# bumps up VERSION file to <upstream-version>-rhobs<patch>
# e.g. upstream 1.2.3 will be bumped to 1.2.3-rhobs1
# and if git tag 1.2.3-rhobs1 already exists, it will be bumped to 1.2.3-rhobs2
bumpup_version() {
	# get all tags with
	header "Bumping up the version"

	git fetch downstream --tags

	local version
	version="$(head -n1 VERSION)"

	# remove any trailing rhobs
	local upstream_version="${version//-rhobs*/}"
	echo "found upstream version: $upstream_version"

	local patch
	# git tag | grep "^v$upstream_version-rhobs" | wc -l
	# NOTE: grep || true prevents grep from setting non-zero exit code
	# if there are no -rhobs tag

	patch="$(git tag | { grep "^v$upstream_version-rhobs" || true; } | wc -l)"
	((patch += 1))

	local rhobs_version="$upstream_version-rhobs$patch"

	echo "Updating version to $rhobs_version"
	echo "$rhobs_version" >VERSION

	ok "version set to $rhobs_version"
}

generate_stripped_down_crds() {
	header "Generating stripped-down CRDs"

	mkdir -p example/stripped-down-crds
	make stripped-down-crds.yaml
	mv stripped-down-crds.yaml example/stripped-down-crds/all.yaml
}

change_api_group() {
	header "Changing api group to monitoring.rhobs"

	rm -f example/prometheus-operator-crd-full/monitoring.coreos.com*
	rm -f example/prometheus-operator-crd/monitoring.coreos.com*

	# NOTE: find command changes
	#  * kubebuilder group to monitoring.rhobs
	#  * the category  to rhobs-prometheus-operator
	#  * removes all shortnames

	find \( -path "./.git" \
		-o -path "./Documentation" \
		-o -path "./rhobs" \) -prune -o \
		-type f -exec \
		sed -i \
		-e 's|monitoring.coreos.com|monitoring.rhobs|g' \
		-e 's|+kubebuilder:resource:categories="prometheus-operator".*|+kubebuilder:resource:categories="rhobs-prometheus-operator"|g' \
		-e 's|github.com/prometheus-operator/prometheus-operator|github.com/rhobs/obo-prometheus-operator|g' \
		{} \;

	# replace only the api group in docs and not the links
	find ./Documentation \
		-type f -exec \
		sed -i -e 's|monitoring.coreos.com|monitoring.rhobs|g' \
		{} \;

	sed -e 's|monitoring\\.coreos\\.com|monitoring\\.rhobs|g' -i .mdox.validate.yaml

	ok "Changed api group to monitoring.rhobs"

	change_go_mod
}

# fix version of downstream imports in go.mod files
# e.g. replace
#  require (
#      github.com/rhobs/obo-prometheus-operator v0.64.0
#      github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring v0.64.0
#      github.com/rhobs/obo-prometheus-operator/pkg/client v0.64.0
#  )
# with
#  require (
#      github.com/rhobs/obo-prometheus-operator v0.64.0-rhobs1
#      github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring v0.64.0-rhobs1
#      github.com/rhobs/obo-prometheus-operator/pkg/client v0.64.0-rhobs1
#  )
change_go_mod() {

	header "Updating go.mod files to require obo-prometheus-operator version $RHOBS_VERSION"

	# remove trailing rhobs
	local upstream_version="v${RHOBS_VERSION//-rhobs*/}"
	local rhobs_version="v${RHOBS_VERSION}"

	# fix import paths
	# NOTE: this step is run after running change_api_group which already changes
	# github.com/prometheus-operator to github.com/rhobs
	find \( -path "./.git" \
		-o -path "./Documentation" \
		-o -path "./rhobs" \) -prune -o \
		-type f -name go.mod -exec \
		sed -i \
		-e "s|github.com/rhobs/obo-prometheus-operator ${upstream_version}$|github.com/rhobs/obo-prometheus-operator ${rhobs_version}|g" \
		-e "s|github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring ${upstream_version}$|github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring ${rhobs_version}|g" \
		-e "s|github.com/rhobs/obo-prometheus-operator/pkg/client ${upstream_version}$|github.com/rhobs/obo-prometheus-operator/pkg/client ${rhobs_version}|g" \
		{} \;

	# tidy up
	find \( -path "./.git" \
		-o -path "./Documentation" \
		-o -path "./rhobs" \) -prune -o \
		-type f -name go.mod -execdir go mod tidy \; || {
		warn "go mod tidy failed"
		return 1
	}

	# update import test case go.mod
	(
		cd rhobs/test/import
		go mod edit -require github.com/rhobs/obo-prometheus-operator@"${rhobs_version}"
		go mod edit -require github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring@"${rhobs_version}"
		go mod edit -require github.com/rhobs/obo-prometheus-operator/pkg/client@"${rhobs_version}"
	)

	ok "go.mod files updated"

	return 0
}

change_container_image_repo() {
	local to_repo="$1"
	shift

	header "Changing container repo from quay.io/prometheus -> $to_repo"

	find \( -path "./.git" \
		-o -path "./Documentation" \
		-o -path "./rhobs" \) -prune -o \
		-type f -exec sed -i \
		-e "s|quay.io/prometheus-operator/|${to_repo}|g" \
		{} \;

	# reset reference to alert manager webhook test images used in tests
	# back to use prometheus-images itself

	info "reset images used for testing"

	find ./test -type f -exec sed -i \
		-e "s|$to_repo/obo-prometheus-alertmanager-test-webhook|quay.io/prometheus-operator/prometheus-alertmanager-test-webhook|g" \
		-e "s|$to_repo/obo-instrumented-sample-app|quay.io/prometheus-operator/instrumented-sample-app|g" \
		{} \;

	ok "Changed container repo to $to_repo"

}

remove_upstream_release_workflows() {
	header "Removing upstream only release workflows"

	git rm -f .github/workflows/release.yaml \
		.github/workflows/stale.yaml \
		.github/workflows/publish.yaml
}

validate_git_repos() {
	header "Validating git remotes"

	local num_remotes
	num_remotes=$(git remote | wc -l)

	local failed=false

	[[ "$num_remotes" -ge 3 ]] || {
		warn "expected to find more than 3 remotes but found only $num_remotes"
		failed=true
	}

	assert_repo_url upstream "prometheus-operator/prometheus-operator" || ((fails++))
	assert_repo_url downstream "rhobs/obo-prometheus-operator" || ((fails++))

	local rhobs_rel_branch="rhobs-rel-${PREVIOUS_VERSION}"

	git ls-remote --exit-code --heads downstream "$rhobs_rel_branch" || {
		warn "invalid previous release version - $PREVIOUS_VERSION "
		failed=true
	}

	if $failed; then
		return 1
	fi

	ok "git remotes looks fine"
	ok "$PREVIOUS_VERSION exists"

	return 0
}

validate_args() {
	header "Validating args ..."

	[[ "$PREVIOUS_VERSION" == "" ]] && {
		warn "wrong usage: must pass --previous-version <version>"
		return 1
	}
	return 0
}

validate_repo_state() {
	$IGNORE_REPO_STATE && {
		info "skipping validation of repo state"
		return 0
	}

	[[ -z "$(git status --porcelain)" ]] || {
		warn "git: repo has local changes; ensure git status is clean"
		return 1
	}

	ok "Repo state looks fine"
	return 0
}

print_usage() {
	local app
	app="$(basename "$0")"

	read -r -d '' help <<-EOF_HELP || true
		        Usage:
		          $app  --previous-version VERSION
		          $app  -h|--help

		        Example:
		          # To upgrade from 0.59.2-rhobs1 version to 0.60.0, run
		          ❯ $app  --previous-version 0.59.2-rhobs1

		        Options:
		          -h|--help               show this help
		          --no-check              skip make checks
		          --ignore-repo-state     run script even if the local repo's state isn't clean

EOF_HELP

	echo -e "$help"
	return 0
}

validate() {
	local failed=false

	validate_args || failed=true
	validate_git_repos || failed=true
	validate_repo_state || failed=true

	if $failed; then
		return 1
	fi
	return 0
}

make_required_targets() {
	header "Running format and generate"
	make --always-make format generate
	make --always-make docs
}

git_release_commit() {

	header "Adding release commit"

	git add .

	local version
	version="$(head -n1 VERSION)"

	git commit -s -F- <<-EOF_COMMIT_MSG
		chore(release): v${version}

		NOTE: This commit was auto-generated by
		running rhobs/$(basename "$0") script
EOF_COMMIT_MSG

}

run_checks() {
	header "Running checks"
	if ! $RUN_MAKE_CHECKS; then
		warn "Skipping make checks"
		return 0
	fi

	make check-docs check-golang check-license check-metrics
	make test-unit
}

parse_args() {
	### while there are args parse them
	while [[ -n "${1+xxx}" ]]; do
		case $1 in
		-h | --help)
			SHOW_USAGE=true
			break
			;; # exit the loop
		--no-checks)
			RUN_MAKE_CHECKS=false
			shift
			;;
		--ignore-repo-state)
			IGNORE_REPO_STATE=true
			shift
			;;
		--previous-version)
			shift
			# only accept the args match VERSION format
			if [[ -n "${1+xxx}" ]] && [[ "$1" =~ [0-9]+.*-rhobs[0-9].* ]]; then
				PREVIOUS_VERSION="$1"
				shift
			fi
			;;
		*)
			warn "unknown arg $1"
			return 1
			;;
		esac
	done

	return 0
}

assert_repo_url() {
	local remote="$1"
	shift
	local expected_url="$1"
	shift

	local actual_url
	actual_url=$(git remote get-url "$remote")

	[[ "$actual_url" =~ $expected_url ]] || {
		warn "git remote '$remote' must point to $expected_url instead of $actual_url"
		return 1
	}

	return 0
}

change_po_gh_urls() {
	local rhobs_rel_branch="rhobs-rel-${PREVIOUS_VERSION}"
	local prev_release_git_branch="https://raw.githubusercontent.com/rhobs/obo-prometheus-operator/${rhobs_rel_branch}"
	local prev_stable_version="${prev_release_git_branch}/VERSION"
	local prev_example_dir="${prev_release_git_branch}/example"
	local prev_resource_dir="${prev_release_git_branch}/test/framework/resources"

	sed \
		-e "s|\(prometheusOperatorGithubBranchURL := .*$\)|// \1|g" \
		-e "s|prevStableVersionURL := .*|prevStableVersionURL := \"${prev_stable_version}\"|g" \
		-e "s|prevExampleDir := .*|prevExampleDir := \"${prev_example_dir}\"|g" \
		-e "s|prevResourcesDir := .*|prevResourcesDir := \"${prev_resource_dir}\"|g" \
		-i test/e2e/main_test.go
}

make_olm_bundle() {
	local image_repo="$1"
	shift
	#
	(
		cd rhobs/olm
		make tools
		rm -rf bundle && make bundle IMAGE_REPO="$image_repo"
	)
}

main() {
	# all files references must be relative to the root of the project
	cd "$PROJECT_ROOT"

	parse_args "$@" || {
		print_usage
		exit 1
	}

	if $SHOW_USAGE; then
		print_usage
		return 0
	fi

	validate || {
		die "Please fix failures ☝️ (indicated by ⚠️ ) and run the script again "
	}

	bumpup_version
	RHOBS_VERSION="$(head -n1 VERSION)"

	change_po_gh_urls
	change_api_group || {
		die "Please fix failures ☝️ (indicated by ⚠️ ) and run the script again "
	}
	change_container_image_repo "$IMG_REPO/$IMG_PREFIX"
	make_required_targets
	generate_stripped_down_crds
	remove_upstream_release_workflows
	make_olm_bundle "$IMG_REPO"
	git_release_commit
	run_checks

	git diff --shortstat --exit-code
}

main "$@"
