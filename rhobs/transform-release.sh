#!/usr/bin/env bash
# Sourced by release.sh after validating the plan and selecting a clean checkout.
[[ "${BASH_SOURCE[0]}" != "$0" ]] || { echo "Use rhobs/release.sh transform --plan FILE" >&2; exit 1; }

assert_clean
source_commit=$(meta .source_commit)
scripts_commit=$(meta .scripts_commit)
version=$(meta .version)
previous=$(meta .previous_version)
git merge-base --is-ancestor "$source_commit" HEAD || die "Release is not based on the pinned source commit"
[[ "$(git rev-parse HEAD:rhobs)" == "$(git rev-parse "$scripts_commit:rhobs")" ]] ||
	die "RHOBS overlay does not match scripts_commit"
[[ "$(head -n1 VERSION)" == "$(meta .source_version)" ]] || die "Source VERSION no longer matches the plan"
check_source_vendor "$source_commit"

git ls-files -z > "$WORK/source-paths"
while IFS= read -r -d '' file; do
	case "$file" in vendor/*|rhobs/*) continue;; esac
	[[ ! -L "$file" ]] || continue
	# Do not run text substitutions over binary files.
	if ! LC_ALL=C grep -Iq . -- "$file"; then continue; fi
	sed -i 's|monitoring\.coreos\.com|monitoring.rhobs|g' -- "$file"
	if [[ "$file" == Documentation/* ]]; then continue; fi
	sed -i \
		-e "s|github\\.com/prometheus-operator/prometheus-operator|$MODULE|g" \
		-e 's|quay.io/prometheus-operator/|quay.io/rhobs/obo-|g' \
		-e 's|+kubebuilder:resource:categories="prometheus-operator".*|+kubebuilder:resource:categories="rhobs-prometheus-operator"|g' -- "$file"
	if [[ "$file" == test/* ]]; then
		sed -i \
			-e 's|quay.io/rhobs/obo-prometheus-alertmanager-test-webhook|quay.io/prometheus-operator/prometheus-alertmanager-test-webhook|g' \
			-e 's|quay.io/rhobs/obo-instrumented-sample-app|quay.io/prometheus-operator/instrumented-sample-app|g' -- "$file"
	fi
done < "$WORK/source-paths"

for directory in example/prometheus-operator-crd example/prometheus-operator-crd-full; do
	if [[ -d "$directory" ]]; then find "$directory" -maxdepth 1 -name 'monitoring.coreos.com*' -type f -delete; fi
done
if [[ -f .mdox.validate.yaml ]]; then
	sed -i 's|monitoring\\\.coreos\\\.com|monitoring\\.rhobs|g' .mdox.validate.yaml
fi

# Self-module requirements can differ from VERSION on an OpenShift branch.
# Replace by exact module identity, retaining unrelated pins and all replaces.
module_re='github\.com/rhobs/obo-prometheus-operator'
while IFS= read -r -d '' file; do
	case "$file" in vendor/*|rhobs/*) continue;; go.mod|*/go.mod) ;; *) continue;; esac
	sed -i -E "s#^([[:blank:]]*(require[[:blank:]]+)?)(${module_re}(/pkg/(apis/monitoring|client))?)[[:blank:]]+v[^[:blank:]]+#\1\3 v${version}#" -- "$file"
done < "$WORK/source-paths"
sed -i -E "s#^([[:blank:]]*(require[[:blank:]]+)?)(${module_re}(/pkg/(apis/monitoring|client))?)[[:blank:]]+v[^[:blank:]]+#\1\3 v${version}#" rhobs/test/import/go.mod
sed -i -E '/^[[:blank:]]*-[[:blank:]]+misspell[[:blank:]]*(#.*)?$/d' .golangci.yml
awk -f "$SCRIPT_DIR/release-makefile.awk" Makefile > "$WORK/Makefile"
cat "$WORK/Makefile" > Makefile

previous_url="https://raw.githubusercontent.com/rhobs/obo-prometheus-operator/v$previous"
for variable in prevStableVersionURL prevExampleDir prevResourcesDir; do
	count=$(grep -cE "^[[:blank:]]*$variable := " test/e2e/main_test.go || true)
	[[ "$count" == 1 ]] || die "Source upgrade test changed: could not replace $variable"
	case "$variable" in
	prevStableVersionURL) suffix=/VERSION;; prevExampleDir) suffix=/example;; prevResourcesDir) suffix=/test/framework/resources;;
	esac
	sed -i -E "s|^([[:blank:]]*)$variable :=.*|\1$variable := \"$previous_url$suffix\"|" test/e2e/main_test.go
done
sed -i -E 's|^([[:blank:]]*)(prometheusOperatorGithubBranchURL :=.*)|\1// \2|' test/e2e/main_test.go

# Retain validation; exclude only upstream publication/housekeeping/spelling.
for file in release.yaml publish.yaml stale.yaml automerge-dependabot.yaml spell-check.yaml cspell.json; do
	rm -f -- ".github/workflows/$file"
done
ci_commit=$(sync_ci "$source_commit")
awk '!/^TAG=/' .github/env > "$WORK/env"
printf 'TAG=v%s\n' "$version" >> "$WORK/env"
cat "$WORK/env" > .github/env
printf '%s\n' "$version" > VERSION
jq --arg ci "$ci_commit" 'del(.changed) | .ci_commit=$ci' "$PLAN" > "$WORK/metadata.json"
cp -- "$WORK/metadata.json" rhobs/release.json
