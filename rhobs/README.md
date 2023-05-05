# RHOBS Scripts

This directory hosts scripts that helps with creation of forked Prometheus
Operator with only the api-group changed to `monitoring.rhobs`

## Making a Release

In this example we have the local git repo setup with 3 remotes. Note that the 
`make-release-commit.sh` script assumes the following remote names to be 
present.

  1. `upstream` -> github.com/prometheus-operator/prometheus-operator
  2. `downstream` -> github.com/rhobs/obo-prometheus-operator
  3. `origin `-> github.com/<your-fork-of>/obo-prometheus-operator

### Create New Release Branch

We start by pushing an already released version of upstream prometheus-operator
to our `downstream` fork (under rhobs org). Note that the downstream release
branches follow nomenclature different to upstream so that the upstream github
worflows don't trigger accidently.

The naming convention used is `rhobs-rel-<upstream-release>-rhobs<patch>`

In this example, we are making a downstream release of `v0.60.0`. Start by
creating a release branch as follows


```
git fetch upstream --tags
git push downstream +v0.60.0^{commit}:refs/heads/rhobs-rel-0.60.0-rhobs1
```

### Make Release Commit

Start by creating a branch for release (`pr-for-release`) and reseting it to
the upstream release version/tag.

```
git checkout -b pr-for-release
git reset --hard v0.60.0
```

Merge the `rhobs-scripts` branch, squashing all its commits into one.

```
git merge --squash --allow-unrelated-histories downstream/rhobs-scripts
git commit -m "git: merge rhobs-scripts"
```

Run the `make-release-commit.sh` script which creates a git commit that
contains all changes required to create the forked prometheus operator for
Observabilty Operator (ObO). The `make-release-commit.sh` takes a mandatory
argument `--previous-version` which should point to the last stable release of
the fork. This stable version is used in `e2e` tests that are run in CI which
validates if the newer version is upgradable from the `previous-version`

```
./rhobs/make-release-commit.sh --previous-version 0.59.2-rhobs1
git push -u origin HEAD
```

### Create Pull Request
Create pull request and ensure that the title says
`chore(release): v0.60.0-rhobs`. This is important since the rhobs-release
(github) workflow makes release iff the commit message starts with
`chore(release)`.

### Automatic release once the PR merges

Check `.github/workflows/rhobs-release.yaml` for details of how the release is
made.
