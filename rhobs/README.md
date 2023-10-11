# RHOBS Scripts

This directory hosts scripts that helps with creation of forked Prometheus
Operator with only differs from the [upstream
repository](https://github.com/prometheus-operator/prometheus-operator) by the
API Group being `monitoring.rhobs` instead of `monitoring.coreos.com`.

## Making a Release

In this example we have our local Git repository setup with 3 remotes. Note
that the `make-release-commit.sh` script assumes the following remote names to
be present.

  1. `upstream` -> `github.com/prometheus-operator/prometheus-operator`
  2. `downstream` -> `github.com/rhobs/obo-prometheus-operator`
  3. `origin `-> `github.com/<your-fork-of>/obo-prometheus-operator`

### Create New Release Branch

We start by pushing an already released version of upstream prometheus-operator
to our `downstream` fork (under the [rhobs](https://github.com/rhobs)
organization). Note that the downstream release branches follow a nomenclature
which is different to upstream so that the upstream github worflows don't
trigger accidently.

The naming convention used is `rhobs-rel-<upstream-release>-rhobs<patch>`

In this example, we are making a downstream release of `v0.60.0`. Start by
creating a release branch as follows:

```bash
export UPSTREAM_VERSION=0.60.0
export CURRENT_DOWNSTREAM_VERSION=0.59.2-rhobs1
git fetch upstream --tags
git push downstream "+v${UPSTREAM_VERSION}^{commit}:refs/heads/rhobs-rel-${UPSTREAM_VERSION}-rhobs1"
```

### Make Release Commit

Start by creating a local branch for the release (e.g. `pr-for-release`) and reseting it to
the upstream release version/tag.

```bash
git checkout -b pr-for-release
git reset --hard "v${UPSTREAM_VERSION}"
```

Merge the `rhobs-scripts` branch, squashing all its commits into one.

```bash
git merge --squash --allow-unrelated-histories downstream/rhobs-scripts
git commit -m "git: merge rhobs-scripts"
```

Run the `make-release-commit.sh` script which creates a Git commit that
contains all changes required to create the forked prometheus operator for
Observabilty Operator (ObO). The `make-release-commit.sh` takes a mandatory
argument `--previous-version` which should point to the last stable release of
the fork. This stable version is used in `e2e` tests that are run in CI which
validates if the newer version is upgradable from the `previous-version`

```bash
./rhobs/make-release-commit.sh --previous-version ${CURRENT_DOWNSTREAM_VERSION}
git push -u origin HEAD
```

### Create Pull Request

Create a pull request against the `rhobs-rel-0.60.0-rhobs1` branch and ensure
that the title says `chore(release): v${UPSTREAM_VERSION}-rhobs1`. This is
important since the rhobs-release (GitHub) workflow makes release if and only
if the commit message starts with `chore(release)`.

### Automatic release once the PR merges

Check `.github/workflows/rhobs-release.yaml` for details of how the release is
made.
