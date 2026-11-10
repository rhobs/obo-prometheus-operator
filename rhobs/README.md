# RHOBS releases

The `rhobs-scripts` branch contains the release overlay, not a buildable operator.
Releases are generated from **openshift/prometheus-operator**, preserving its
fixes while changing the API group to `monitoring.rhobs`, Go module paths to
`github.com/rhobs/obo-prometheus-operator`, and image references to RHOBS.
The overlay also supplies the RHOBS/ART Dockerfiles and RHOBS-owned CI.

## Release series and versions

- A **new series** selects the highest numerically sorted `release-x.y` branch
  in `openshift/prometheus-operator`. Only exact branch names match; tags and
  commit dates do not influence selection. This intentionally includes the
  highest development release branch, not just GA branches.
- The operator's `VERSION` at the selected commit determines the base version.
  For example, OpenShift `release-5.2` can contain operator `0.94.2`.
- The first downstream release is `v0.94.2-rhobs1`; subsequent releases use the
  greatest published revision **plus one**, not the number of existing tags.
  Existing versions are never reused, even across OpenShift branches containing
  the same operator version.
- A **rebuild** stays on the source repository, branch, and base operator version
  recorded by the previous release. It never upgrades to a newer OpenShift
  branch automatically. No source change means no rebuild unless `--force` is
  explicitly requested (for example, to pick up an updated overlay/base image).
- If the source branch's operator `VERSION` changes, explicitly start a new
  series. Missing branches, rewritten history, downgrades, and stale revision
  baselines fail rather than silently selecting a different source.

The legacy `-rhobsN` suffix is retained. It is a SemVer prerelease identifier:
consumers should pin explicit versions, rather than relying on prerelease
lexical ordering (`rhobs10` versus `rhobs9`). The release tooling sorts revisions
numerically.

Every generated release records the source repository, branch, immutable source
SHA, original/base operator version, downstream version, previous release, and
overlay SHA in `rhobs/release.json`. The generated release also records the
upstream workflow snapshot as `ci_commit`. A source tag is **not** required.

## Prerequisites

- Git, Go 1.22+ for the standalone release tool, Bash, Make, GCC (race tests),
  wget, and jq. Operator builds also need Go compatible with the selected
  source's `go.mod` / `.github/env`.
- A configured Git name/email; generated commits are signed off with `git -s`.
- For PR publication only: `gh`, GitHub authentication, write access to the
  downstream release base, and a personal fork for the PR branch.
- For image publication/E2E only: Docker with Buildx; E2E also needs Kind/kubectl.

The Makefiles install pinned generators, linters, and OLM tools into `tmp/bin/`.
The release CLI is a standard-library-only Go module under `rhobs/release/`.
`bash rhobs/release.sh` runs it independently of the operator's modules,
workspace, and vendor tree. CI builds it once and selects that binary through
`RHOBS_RELEASE_BIN`; local use needs no installation step.
Keep the usual remotes: `origin` = personal fork, `upstream` = canonical RHOBS
repository. No renaming or token-bearing remote URLs are required; the release
tool accepts repository URLs directly.

Commit overlay changes before preparing a release: `--scripts-ref` defaults to
the current commit, and preparation requires a clean checkout.

## Plan a new series

From a checkout of `rhobs-scripts`:

```bash
bash rhobs/release.sh plan --mode new > /tmp/opencode/rhobs-release-plan.json
cat /tmp/opencode/rhobs-release-plan.json
```

The numerically latest published **root downstream tag** supplies the upgrade
baseline by default. Pass `--previous-version 0.93.1-rhobs1` to select another
supported baseline. A branch without a published tag is not a released version.

Planning only fetches objects into private `refs/rhobs/` references. It does not
create branches, alter the checkout, fetch upstream tags, or push anything.
Review the source SHA and versions before execution.

## Prepare a local release

```bash
bash rhobs/prepare-release.sh --plan /tmp/opencode/rhobs-release-plan.json --execute
```

An isolated worktree is created (use `--worktree PATH` to choose its location):

1. Start `rhobs-pr-<version>` at the exact source SHA.
2. Apply only the pinned overlay's tracked files and commit them. This avoids
   unrelated-history merge conflicts when OpenShift retains upstream CI files.
3. Apply the RHOBS transformations and write the release metadata.
4. Restore upstream CI from the selected source/history, apply `rhobs/ci.patch`,
   then generate clients, CRDs, metrics, examples, stripped CRDs, and the OLM bundle.
5. Reconcile vendoring while retaining the source's dependency pins/replacements.
   Fail if **any third-party vendor file** is lost or changed; investigate
   OpenShift-only vendor patches rather than silently overwriting them.
6. Run lint, unit, and local-module import checks; create a signed-off
   `chore(release): v<version>` commit.

The original checkout is untouched. Failed worktrees are retained for diagnosis.
`--no-checks` skips optional local checks, not generation or mandatory CI.

Without `--execute`, preparation only prints the plan. A previously saved plan
pins both the source and overlay: preparation does not discover a different tip.
It rejects a plan whose version has since been published.

## Open the release PR

```bash
gh auth setup-git
bash rhobs/prepare-release.sh --plan /tmp/opencode/rhobs-release-plan.json \
  --execute --publish-pr --fork-repository jan--f/obo-prometheus-operator
```

This explicitly authorizes pushing the pristine source SHA as
`rhobs-rel-<version>`, pushing the generated PR branch to the fork, and opening a
PR titled `chore(release): v<version>`. Nothing is force-pushed. An existing open
PR for that revision is reused. Use the PR title for the squash merge so the
release commit convention is retained.

Alternatively, prepare locally and push/open the PR manually. The target base
must be created from the pinned source SHA, not an upstream tag.

## Periodic rebuilds

```bash
bash rhobs/release.sh plan --mode rebuild > /tmp/opencode/rhobs-release-plan.json
bash rhobs/prepare-release.sh --plan /tmp/opencode/rhobs-release-plan.json --execute
```

Use `--previous-version` to rebuild an older supported series; that tag must be
the latest published revision of its base operator version. Legacy releases
without provenance can be the upgrade baseline for a **new** series, but cannot
be implicitly rebuilt. `--source-commit FULL_SHA` can select an exact snapshot
on the selected source branch; rebuilds cannot go backwards in source history.

### GitHub automation deployment

The workflows live on `rhobs-scripts` and are included in each generated release.
**GitHub schedules and dispatch registration require workflows on the default
branch.** Install `.github/workflows/rhobs-prepare-release.yaml` on `rhobs-main`
in a separate, small default-branch change; do not merge the entire overlay into
that lightweight branch. The controller checks out `rhobs-scripts` itself.

Configure:

- Secret `RHOBS_RELEASE_TOKEN`: a bot/PAT able to push to the downstream/fork and
  open PRs. The default `GITHUB_TOKEN` must not be used for these pushes, because
  its events do not trigger the PR's validation workflows.
- Variable `RHOBS_RELEASE_FORK`: bot/contributor `owner/repo` for PR branches.
- Variable `RHOBS_AUTOMATIC_REBUILDS=true`: opt in **after** validating the first
  new-series release. Before then the weekly schedule is disabled.

Manual dispatch defaults to a dry run. The controller uses a bootstrap Go
toolchain to resolve one JSON plan and read its source Go version. An executing
run installs that selected toolchain and reuses the exact plan to prepare a PR.
Scheduled runs rebuild the latest published series and do nothing for an
unchanged source SHA. They do not merge PRs or publish releases themselves.

## Reused upstream validation

Operator CI is upstream code, not a parallel RHOBS implementation. The checked-in
`checks.yaml`, `unit.yaml`, `e2e.yaml`, feature-gate, Prometheus-upgrade/v2,
changed-files, and Actionlint workflows are generated from OpenShift's upstream
definitions. Each file identifies its exact source commit. The shared
`.github/actions/setup-e2e/action.yaml` is copied **unchanged** from the selected
OpenShift source snapshot.

When OpenShift has removed `.github/workflows`, release preparation finds the
nearest first-parent source snapshot that still contains them. It applies the
reviewable delta in `rhobs/ci.patch`. A patch conflict fails with an instruction
to update that delta; it never silently falls back to custom CI.

The delta is limited to downstream integration:

- Release-PR triggers and reusable workflow calls, rather than upstream tag
  publication or schedules on the lightweight default branch.
- Read-only permissions, distinct reusable-workflow concurrency groups, and
  correct changed-file handling for both PRs and publication pushes.
- Linux-only builds, all shipped binaries, no documentation/link or spelling
  checks, and suite-specific diagnostic artifact names.

The existing Makefile `format generate` recipe is reused by release preparation
and upstream checks. Transformation removes only its documentation prerequisites
(`docs` and `generate-docs`), excludes vendor files from formatting, and disables
golangci `misspell`. All other upstream lint/generation/unit/extended/E2E jobs
remain upstream-owned. `.github/env` supplies the downstream version image tag,
so the unchanged E2E setup builds/loads the correct RHOBS images. Upgrade tests
reference the previous immutable downstream release.

Only `rhobs-checks.yaml` remains custom: it tests the release tooling and checks
RHOBS-specific provenance, vendoring, stripped CRDs, OLM bundle, and local Go
imports. There is no custom E2E matrix, suite selector, or duplicate unit/lint CI.
Periodic rebuild PRs run all reused upstream suites before publication; no
separate downstream E2E scheduler needs to be maintained.

### Updating the upstream snapshot

For a source commit already fetched by planning:

```bash
bash rhobs/release.sh sync-ci --ref SOURCE_COMMIT
git diff -- .github/workflows .github/actions/setup-e2e
actionlint
```

This refresh is also automatic during release preparation. Maintain
`rhobs/ci.patch`, not the generated workflow bodies. It excludes upstream release,
publish, stale, dependency-automerge, and spelling workflows without blanket
deletion of other source workflows.

For branch protection, use the reused workflows' checks (including upstream
`End-to-End Test Results`) together with `RHOBS validation result`. Publication
requires all reusable workflows to succeed for its exact commit.

## Publication and retries

`rhobs-release.yaml` only publishes conventionally named release commits on
`rhobs-rel-**` in the canonical repository. It calls the required checks and all
E2E suites against the exact commit **before** receiving publication credentials.
Publication is serialized and can be rerun manually after a failure.

Images/bundle are built under commit-specific staging tags, labeled with their
release commit, and promoted to missing release tags. Existing version tags are
only accepted if their image revision matches the same commit; they are never
overwritten. A retry can complete partial publication. Registry errors other
than a missing manifest fail rather than being treated as a missing release.

Exactly three annotated Git tags are pushed atomically:

```text
v<version>
pkg/apis/monitoring/v<version>
pkg/client/v<version>
```

Existing matching tags are reused; conflicting tags fail. `git push --tags` is
never used. A post-publication smoke test imports the published modules, without
local replacements. Registry and Git publication are not one atomic transaction;
the staging/retry checks recover an interrupted publication without replacing
already released artifacts.

## Test the overlay tooling

```bash
bash rhobs/ci.sh tools
actionlint
```

The tooling checks run `go test -race`, `go vet`, formatting, and ShellCheck.
The Go tests use disposable local Git repositories and Go-based fake registry
commands; they do not contact GitHub or Quay. They also cover restoring/patching
upstream CI, publication's changed-file gate, and pinned Go-version helpers.
No release credentials are needed.
Full operator generation/build/E2E validation requires a prepared source
checkout, not `rhobs-scripts` itself.
