#!/usr/bin/env bash
# Local integration tests: disposable Git repositories, Bash/jq fakes, no network.
set -euo pipefail
shopt -s inherit_errexit
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
RELEASE="$ROOT/rhobs/release.sh"
if [[ -z "${TMPDIR:-}" && -d /tmp/opencode ]]; then export TMPDIR=/tmp/opencode; fi

fail() { echo "FAIL: $*" >&2; exit 1; }
eq() { [[ "$1" == "$2" ]] || fail "got '$1', expected '$2'"; }
contains() { grep -qF -- "$2" "$1" || fail "$1 lacks '$2'"; }
expect_failure() {
	if "$@" > "$T/failure.out" 2> "$T/failure.err"; then fail "Unexpected success: $*"; fi
}
write() { mkdir -p -- "$(dirname -- "$1")"; printf '%s' "$2" > "$1"; }
commit() {
	git -C "$1" add --all
	git -C "$1" commit -qs -m "$2" -m 'Assisted-by: opencode (gpt-6.1-sol)'
	git -C "$1" rev-parse HEAD
}
init_repo() {
	git init -q -b main "$1"
	git -C "$1" config user.name 'Release Test'
	git -C "$1" config user.email release-test@example.invalid
	git -C "$1" config commit.gpgsign false
	git -C "$1" config tag.gpgsign false
}

fixture() {
	T="$(mktemp -d "${TMPDIR:-/tmp}/rhobs-test.XXXXXX")"
	trap 'rm -rf -- "$T"' EXIT
	export GITHUB_REF=""
	SRC="$T/source"; DOWN="$T/downstream"; CONTROL="$T/controller"; TREE="$T/prepared"
	init_repo "$SRC"; init_repo "$DOWN"; init_repo "$CONTROL"
	# Reverse our delta to obtain upstream fixtures without a second vendored copy.
	mkdir -p "$T/upstream/.github/workflows"
	for file in checks.yaml unit.yaml changed-files.yaml e2e.yaml e2e-feature-gated.yaml \
		test-prom-version-upgrade.yaml e2e-prometheus-v2.yaml actionlint.yml; do
		sed '1,3d' "$ROOT/.github/workflows/$file" > "$T/upstream/.github/workflows/$file"
	done
	git -C "$T/upstream" -c apply.whitespace=nowarn apply --reverse "$ROOT/rhobs/ci.patch"
	cp -a "$T/upstream/.github" "$SRC/"
	mkdir -p "$SRC/.github/actions/setup-e2e"
	cp -- "$ROOT/.github/actions/setup-e2e/action.yaml" "$SRC/.github/actions/setup-e2e/action.yaml"
	write "$SRC/VERSION" $'0.94.2\n'
	write "$SRC/.github/env" $'golang-version=1.26\n'
	write "$SRC/.golangci.yml" $'version: "2"\nlinters:\n  enable:\n    - misspell\n    - revive\n'
	write "$SRC/go.mod" $'module github.com/prometheus-operator/prometheus-operator\ngo 1.26.0\nrequire (\n github.com/prometheus-operator/prometheus-operator/pkg/client v0.94.1\n github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.94.1\n)\nreplace github.com/prometheus-operator/prometheus-operator/pkg/client => ./pkg/client\n'
	write "$SRC/pkg/client/go.mod" $'module github.com/prometheus-operator/prometheus-operator/pkg/client\ngo 1.26.0\nrequire github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.94.1\n'
	write "$SRC/pkg/client/client.go" $'package client\n// monitoring.coreos.com\n'
	write "$SRC/vendor/github.com/prometheus-operator/prometheus-operator/pkg/client/client.go" $'package client\n// monitoring.coreos.com\n'
	write "$SRC/vendor/example.org/dependency/patch.go" $'package dependency\n// OpenShift-only fix\n'
	write "$SRC/Documentation/example.md" $'monitoring.coreos.com https://github.com/prometheus-operator/prometheus-operator/issues/1\n'
	write "$SRC/example/prometheus-operator-crd/monitoring.coreos.com_prometheuses.yaml" $'old CRD\n'
	write "$SRC/.mdox.validate.yaml" 'monitoring\.coreos\.com'
	write "$SRC/Makefile" $'format: go-fmt jsonnet-fmt fix-license shellcheck docs\ngenerate: k8s-gen generate-metrics generate-docs image-builder-version\ngo-fmt:\n\tgofmt -s -w .\njsonnet-fmt:\n\tfind . -name *.jsonnet -or -name *.libsonnet -not -path "*/vendor/*" -print0 | xargs -0 $(JSONNETFMT_BINARY) -i\noperator-image:\n\t$(CONTAINER_CLI) build -t operator .\n'
	write "$SRC/test/e2e/main_test.go" $'package e2e\nfunc fixture() {\n prevStableVersionURL := "upstream/VERSION"\n prevExampleDir := "upstream/example"\n prevResourcesDir := "upstream/resources"\n}\n'
	commit "$SRC" 'upstream CI' >/dev/null
	printf '\n# A newer upstream unit-test change.\n' >> "$SRC/.github/workflows/unit.yaml"
	CI_COMMIT=$(commit "$SRC" 'update another workflow')
	find "$SRC/.github/workflows" -type f -delete
	SOURCE_COMMIT=$(commit "$SRC" 'OpenShift removes unused CI')
	git -C "$SRC" branch release-4.9
	git -C "$SRC" checkout -qb release-5.2
	write "$DOWN/VERSION" $'0.93.1-rhobs1\n'
	LEGACY=$(commit "$DOWN" 'legacy downstream')
	git -C "$DOWN" tag -a v0.93.1-rhobs1 -m legacy
	cp -a "$ROOT/rhobs" "$ROOT/.github" "$CONTROL/"
	SCRIPTS_COMMIT=$(commit "$CONTROL" 'RHOBS overlay')
	mkdir "$T/bin"
	for tool in docker make go gh; do ln -s "$ROOT/rhobs/test/fake-command.sh" "$T/bin/$tool"; done
	export PATH="$T/bin:$PATH" FAKE_LOG="$T/commands.jsonl" FAKE_REGISTRY="$T/registry.json"
	export FAKE_REGISTRY_ERROR=false FAKE_PRS='[]'
	printf '{}\n' > "$FAKE_REGISTRY"
	: > "$FAKE_LOG"
}

plan() {
	bash "$RELEASE" --root "$CONTROL" plan --mode new --source "$SRC" \
		--downstream "$DOWN" --scripts-ref "$SCRIPTS_COMMIT" "$@" > "$T/plan.json"
}
prepare() {
	bash "$RELEASE" --root "$CONTROL" prepare --plan "$T/plan.json" --downstream "$DOWN" \
		--execute --no-checks --worktree "$TREE" > "$T/prepare.out" 2> "$T/prepare.err"
}
create_release() { plan; prepare; }
publish() { bash "$RELEASE" --root "$TREE" publish-tags --remote "$DOWN"; }
published_release() { create_release; publish; }

test_plan_latest() {
	fixture
	git -C "$SRC" branch release-5.9
	git -C "$SRC" branch release-5.10
	git -C "$SRC" branch release-99.1-rc
	plan
	eq "$(jq -r .source_branch "$T/plan.json")" release-5.10
	eq "$(jq -r .source_commit "$T/plan.json")" "$SOURCE_COMMIT"
	eq "$(jq -r .version "$T/plan.json")" 0.94.2-rhobs1
	eq "$(git -C "$SRC" tag --list)" ""
	eq "$(git -C "$CONTROL" tag --list)" ""
	bash "$RELEASE" --root "$CONTROL" go-version --plan "$T/plan.json" > "$T/go-version"
	eq "$(cat "$T/go-version")" 1.26.0
	[[ ! -s "$FAKE_LOG" ]] || fail 'Planning invoked a build command'
}
test_numeric_revisions() {
	fixture
	git -C "$DOWN" tag v0.94.2-rhobs1
	git -C "$DOWN" tag v0.94.2-rhobs9
	git -C "$DOWN" tag pkg/client/v0.94.2-rhobs99
	plan
	eq "$(jq -r .version "$T/plan.json")" 0.94.2-rhobs10
	git -C "$DOWN" tag v0.94.2-rhobs999999999999999999999999999999
	plan
	eq "$(jq -r .version "$T/plan.json")" 0.94.2-rhobs1000000000000000000000000000000
}
test_partial_revision() {
	fixture
	git -C "$DOWN" tag pkg/client/v0.94.2-rhobs1
	expect_failure bash "$RELEASE" --root "$CONTROL" plan --mode new --source "$SRC" --downstream "$DOWN"
	contains "$T/failure.err" 'Partial publication'
}
test_safety_defaults() {
	fixture; plan
	bash "$RELEASE" --root "$CONTROL" prepare --plan "$T/plan.json" --downstream "$DOWN" > "$T/dry-run"
	eq "$(git -C "$CONTROL" branch --list 'rhobs-pr-*')" ""
	expect_failure bash "$RELEASE" --root "$CONTROL" prepare --mode new --publish-pr
	contains "$T/failure.err" 'requires --execute'
	expect_failure bash "$ROOT/rhobs/transform-release.sh"
	contains "$T/failure.err" 'release.sh transform --plan'
	write "$CONTROL/user-work" 'leave me alone'
	expect_failure prepare
	eq "$(cat "$CONTROL/user-work")" 'leave me alone'
	[[ ! -d "$TREE" ]] || fail 'Dirty controller produced a release worktree'
}
test_prepare_transform() {
	fixture; create_release
	eq "$(jq -r .ci_commit "$TREE/rhobs/release.json")" "$CI_COMMIT"
	eq "$(jq -r 'has("changed")' "$TREE/rhobs/release.json")" false
	eq "$(git -C "$TREE" status --porcelain)" ""
	contains "$TREE/go.mod" 'pkg/client v0.94.2-rhobs1'
	contains "$TREE/pkg/client/go.mod" 'apis/monitoring v0.94.2-rhobs1'
	contains "$TREE/go.mod" '=> ./pkg/client'
	contains "$TREE/pkg/client/client.go" 'monitoring.rhobs'
	contains "$TREE/Documentation/example.md" 'https://github.com/prometheus-operator/prometheus-operator/issues/1'
	contains "$TREE/.mdox.validate.yaml" 'monitoring\.rhobs'
	contains "$TREE/test/e2e/main_test.go" '/v0.93.1-rhobs1/VERSION'
	contains "$TREE/.github/env" 'TAG=v0.94.2-rhobs1'
	contains "$TREE/Makefile" 'format: go-fmt jsonnet-fmt fix-license shellcheck'
	contains "$TREE/Makefile" 'generate: k8s-gen generate-metrics image-builder-version'
	! grep -q misspell "$TREE/.golangci.yml" || fail 'Spelling linter remains enabled'
	cmp "$SRC/vendor/example.org/dependency/patch.go" "$TREE/vendor/example.org/dependency/patch.go"
	cmp "$SRC/.github/actions/setup-e2e/action.yaml" "$TREE/.github/actions/setup-e2e/action.yaml"
	bash "$RELEASE" --root "$TREE" verify > /dev/null
}
test_rebuild_branch_and_noop() {
	fixture; published_release
	git -C "$SRC" branch release-6.0
	plan --mode rebuild
	eq "$(jq -r .changed "$T/plan.json")" false
	plan --mode rebuild --force
	eq "$(jq -r .changed "$T/plan.json")" true
	write "$SRC/fix" 'OpenShift backport'
	fixed=$(commit "$SRC" 'backport fix')
	plan --mode rebuild
	eq "$(jq -r .source_commit "$T/plan.json")" "$fixed"
	eq "$(jq -r .source_branch "$T/plan.json")" release-5.2
	eq "$(jq -r .version "$T/plan.json")" 0.94.2-rhobs2
	# Explicitly starting a new series still cannot reuse rhobs1.
	plan
	eq "$(jq -r .source_branch "$T/plan.json")" release-6.0
	eq "$(jq -r .version "$T/plan.json")" 0.94.2-rhobs2
}
test_legacy_rebuild() {
	fixture
	expect_failure plan --mode rebuild
	contains "$T/failure.err" 'Legacy releases'
}
test_rollover() {
	fixture; published_release
	write "$SRC/VERSION" $'0.95.0\n'; commit "$SRC" 'rollover' >/dev/null
	expect_failure plan --mode rebuild
	contains "$T/failure.err" 'VERSION changed'
}
test_rewritten_history() {
	fixture; published_release
	git -C "$SRC" checkout --orphan rewritten
	commit "$SRC" 'unrelated history' >/dev/null
	git -C "$SRC" branch -f release-5.2 HEAD
	expect_failure plan --mode rebuild
	contains "$T/failure.err" 'history was rewritten'
}
test_missing_branch() {
	fixture; published_release
	git -C "$SRC" checkout main
	git -C "$SRC" branch -D release-5.2
	expect_failure plan --mode rebuild
}
test_pinned_and_stale_plan() {
	fixture; plan
	write "$SRC/fix" 'after the plan'; commit "$SRC" 'later fix' >/dev/null
	prepare
	eq "$(jq -r .source_commit "$TREE/rhobs/release.json")" "$SOURCE_COMMIT"
	publish
	expect_failure bash "$RELEASE" --root "$CONTROL" prepare --plan "$T/plan.json" --downstream "$DOWN"
	contains "$T/failure.err" stale
}
test_vendor_guards() {
	fixture; create_release
	write "$TREE/staged-user-work" 'preserve the real index'
	git -C "$TREE" add staged-user-work
	write "$TREE/vendor/example.org/dependency/patch.go" 'lost fix'
	before=$(git -C "$TREE" diff --cached)
	expect_failure bash "$RELEASE" --root "$TREE" check-vendor
	contains "$T/failure.err" 'third-party vendor'
	eq "$(git -C "$TREE" diff --cached)" "$before"
	cp "$SRC/vendor/example.org/dependency/patch.go" "$TREE/vendor/example.org/dependency/patch.go"
	write "$TREE/vendor/ignored-new-file" 'unexpected dependency'
	expect_failure bash "$RELEASE" --root "$TREE" check-vendor
}
test_vendor_only_self_patch() {
	fixture
	printf '// vendor-only fix\n' >> "$SRC/vendor/github.com/prometheus-operator/prometheus-operator/pkg/client/client.go"
	commit "$SRC" 'patch only the vendored client' >/dev/null
	plan; expect_failure prepare
	contains "$T/prepare.err" 'vendor-only patch'
}
test_tag_publication() {
	fixture; create_release
	git -C "$TREE" tag v99.0.0
	publish; publish
	eq "$(git -C "$DOWN" tag --list '*0.94.2-rhobs1' | wc -l)" 3
	eq "$(git -C "$DOWN" tag --list v99.0.0)" ""
}
test_partial_tag_publication() {
	fixture; create_release
	git -C "$TREE" tag -a v0.94.2-rhobs1 -m partial
	git -C "$TREE" push "$DOWN" refs/tags/v0.94.2-rhobs1
	publish
	eq "$(git -C "$DOWN" tag --list '*0.94.2-rhobs1' | wc -l)" 3
	git -C "$DOWN" tag -d pkg/client/v0.94.2-rhobs1
	expect_failure plan --mode rebuild
	contains "$T/failure.err" 'partial tag publication'
}
test_tag_collision() {
	fixture; create_release
	git -C "$DOWN" tag v0.94.2-rhobs1 "$LEGACY"
	expect_failure publish
	contains "$T/failure.err" 'another commit'
	eq "$(git -C "$DOWN" rev-parse v0.94.2-rhobs1)" "$LEGACY"
}
test_published_imports() {
	fixture; create_release
	cp "$TREE/rhobs/test/import/go.mod" "$T/import-original"
	: > "$FAKE_LOG"
	GOWORK="$T/nonexistent.work" bash "$RELEASE" --root "$TREE" check-import --published
	jq -es 'length == 4 and all(.command=="go" and .gowork=="off")
		and ([.[0:3][] | .args | index("-dropreplace")] | all(. != null))' "$FAKE_LOG" >/dev/null
	cmp "$T/import-original" "$TREE/rhobs/test/import/go.mod"
}
test_ci_patch_failure() {
	fixture
	cp -a "$T/upstream/.github/workflows/." "$SRC/.github/workflows/"
	write "$SRC/.github/workflows/checks.yaml" $'name: incompatible\non:\n  pull_request:\njobs: {}\n'
	broken=$(commit "$SRC" 'upstream restructures CI')
	plan
	before=$(git -C "$CONTROL" status --porcelain)
	expect_failure bash "$RELEASE" --root "$CONTROL" sync-ci --ref "$broken"
	contains "$T/failure.err" 'refresh rhobs/ci.patch'
	eq "$(git -C "$CONTROL" status --porcelain)" "$before"
}
test_ci_retained() {
	fixture
	cp -a "$T/upstream/.github/workflows/." "$SRC/.github/workflows/"
	retained=$(commit "$SRC" 'OpenShift retains upstream CI')
	create_release
	eq "$(jq -r .ci_commit "$TREE/rhobs/release.json")" "$retained"
}
test_existing_pr_reused() {
	fixture; plan
	# Redirect only this fixture's canonical Git URL; never contact GitHub.
	git -C "$CONTROL" config "url.$DOWN.insteadOf" https://github.com/rhobs/obo-prometheus-operator.git
	export FAKE_PRS='[{"html_url":"https://example.invalid/pr/1"}]'
	bash "$RELEASE" --root "$CONTROL" prepare --plan "$T/plan.json" --execute \
		--publish-pr --fork-repository contributor/obo-prometheus-operator > "$T/existing-pr"
	contains "$T/existing-pr" 'https://example.invalid/pr/1'
	eq "$(git -C "$CONTROL" branch --list 'rhobs-pr-*')" ""
}
test_invalid_metadata() {
	fixture; plan
	jq '.source_commit="not-a-commit"' "$T/plan.json" > "$T/invalid.json"
	expect_failure bash "$RELEASE" --root "$CONTROL" transform --plan "$T/invalid.json"
	contains "$T/failure.err" 'invalid release metadata'
	jq 'del(.changed)' "$T/plan.json" > "$T/invalid.json"
	expect_failure bash "$RELEASE" --root "$CONTROL" prepare --plan "$T/invalid.json" --downstream "$DOWN"
	contains "$T/failure.err" 'boolean changed'
}
test_stale_revision_baseline() {
	fixture; published_release
	plan --mode rebuild --force
	TREE="$T/prepared2"; prepare; publish
	expect_failure plan --mode rebuild --previous-version 0.94.2-rhobs1
	contains "$T/failure.err" 'latest published revision'
}
test_image_publication() {
	fixture; create_release
	: > "$FAKE_LOG"
	export IMAGE_ORG=rhobs CPU_ARCHS=amd64 FAKE_COMMIT
	FAKE_COMMIT=$(git -C "$TREE" rev-parse HEAD)
	local image ref
	for image in obo-prometheus-operator obo-prometheus-config-reloader obo-admission-webhook obo-prometheus-operator-bundle; do
		ref="quay.io/rhobs/$image:v0.94.2-rhobs1"
		jq --arg ref "$ref" --arg commit "$FAKE_COMMIT" '.[$ref]=$commit' "$FAKE_REGISTRY" > "$T/state"
		mv "$T/state" "$FAKE_REGISTRY"
	done
	(cd "$TREE"; bash rhobs/publish-images.sh)
	jq -es 'all(.command != "make" and (.args | index("create") | not))' "$FAKE_LOG" >/dev/null
	# Reuse the operator image, promote only the other three missing versions.
	jq 'with_entries(select(.key | contains("/obo-prometheus-operator:")))' "$FAKE_REGISTRY" > "$T/state"
	mv "$T/state" "$FAKE_REGISTRY"; : > "$FAKE_LOG"
	(cd "$TREE"; bash rhobs/publish-images.sh)
	jq -es '[.[] | select(.command=="docker" and .args[2]=="create")
		| .args as $a | $a[($a | index("--tag"))+1]
		| select(endswith(":v0.94.2-rhobs1"))] | length == 3' "$FAKE_LOG" >/dev/null
	# A conflicting existing artifact or registry error must stop before builds.
	: > "$FAKE_LOG"
	printf '{"quay.io/rhobs/obo-prometheus-operator:v0.94.2-rhobs1":"other"}\n' > "$FAKE_REGISTRY"
	# shellcheck disable=SC2016 # $1 is expanded by the child shell.
	expect_failure bash -c 'cd "$1"; bash rhobs/publish-images.sh' _ "$TREE"
	contains "$T/failure.err" 'Refusing to overwrite'
	: > "$FAKE_LOG"; export FAKE_REGISTRY_ERROR=true
	# shellcheck disable=SC2016
	expect_failure bash -c 'cd "$1"; bash rhobs/publish-images.sh' _ "$TREE"
	contains "$T/failure.err" 'authentication required'
	jq -es 'all(.command != "make" and (.args | index("create") | not))' "$FAKE_LOG" >/dev/null
}

if [[ "${1:-}" == --case ]]; then "$2"; exit; fi
tests=(test_plan_latest test_numeric_revisions test_partial_revision test_safety_defaults
	test_prepare_transform test_rebuild_branch_and_noop test_legacy_rebuild test_rollover
	test_rewritten_history test_missing_branch test_pinned_and_stale_plan test_vendor_guards
	test_vendor_only_self_patch test_tag_publication test_partial_tag_publication test_tag_collision
	test_published_imports test_ci_patch_failure test_ci_retained test_existing_pr_reused
	test_invalid_metadata test_stale_revision_baseline test_image_publication)
failed=0
for test in "${tests[@]}"; do
	log=$(mktemp "${TMPDIR:-/tmp}/rhobs-test-output.XXXXXX")
	if bash "$0" --case "$test" > "$log" 2>&1; then echo "PASS $test"
	else echo "FAIL $test"; cat "$log"; failed=$((failed+1)); fi
	rm -f -- "$log"
done
echo "${#tests[@]} tests, $failed failures"
[[ "$failed" == 0 ]]
