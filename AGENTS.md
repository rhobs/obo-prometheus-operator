# Agent Instructions

Repository: `rhobs/obo-prometheus-operator`
Instructions source: `rhobs-scripts` branch, `rhobs/README.md`

The goal is to create a new release when a new version is available in the upstream repo
github.com/prometheus-operator/prometheus-operator

DO NOT make any changes outside of the script run described in the [instructions below](#release-process).

### Prerequisites

Check for the following commands to be available. Install needed packages before 
running the release script:

- `make` — build system
- `wget` — Makefile downloads shellcheck binary internally
- `gcc` — required for `CGO_ENABLED=1` (unit tests use `-race`)
- `shellcheck` — shell linting
- gh — https://cli.github.com/

All other Go tools (jsonnet, jsonnetfmt, jb, controller-gen, golangci-lint, etc.) are installed automatically by the Makefile into `tmp/bin/`.

### Git Setup

When cloning via `gh repo clone <fork>`, the remotes default to `origin` = fork, `upstream` = rhobs/obo-prometheus-operator. The release script expects **different** remote names:

```bash
git remote rename upstream downstream
git remote add upstream https://github.com/prometheus-operator/prometheus-operator.git
```

Required layout:
- `upstream` → `github.com/prometheus-operator/prometheus-operator`
- `downstream` → `github.com/rhobs/obo-prometheus-operator`
- `origin` → `github.com/<your-fork>/obo-prometheus-operator`

For pushing, set authenticated URLs:

```bash
git remote set-url downstream "https://x-access-token:$(gh auth token)@github.com/rhobs/obo-prometheus-operator.git"
git remote set-url origin "https://x-access-token:$(gh auth token)@github.com/<fork>/obo-prometheus-operator.git"
```

If git identity (required for commits) is not set, prompt the user.


### Release Process

Given `UPSTREAM_VERSION` (e.g. `0.89.0`) and `CURRENT_DOWNSTREAM_VERSION` (e.g. `0.87.0-rhobs1`):

```bash
# 1. Fetch upstream tags
git fetch upstream --tags

# 2. Push upstream release commit as new downstream branch
git push downstream "+v${UPSTREAM_VERSION}^{commit}:refs/heads/rhobs-rel-${UPSTREAM_VERSION}-rhobs1"

# 3. Create local working branch at upstream tag
git checkout -b pr-for-release
git reset --hard "v${UPSTREAM_VERSION}"

# 4. Merge rhobs-scripts
git fetch downstream rhobs-scripts
git merge --squash --allow-unrelated-histories downstream/rhobs-scripts
git commit -m "git: merge rhobs-scripts"

# 5. Run the release script
./rhobs/make-release-commit.sh --previous-version ${CURRENT_DOWNSTREAM_VERSION}

# 6. Push to fork
git push -u origin pr-for-release
```

### Finding CURRENT_DOWNSTREAM_VERSION

Check the latest downstream release tag:

```bash
gh api repos/rhobs/obo-prometheus-operator/tags --jq '.[].name' --paginate | grep rhobs | sort -V | tail -1
```

### PR Convention

- Target branch: `rhobs-rel-${UPSTREAM_VERSION}-rhobs1`
- PR title: `chore(release): v${UPSTREAM_VERSION}-rhobs1`
- The title format matters — the `rhobs-release` GitHub workflow triggers on commit messages starting with `chore(release)`.

### Known Issues

- Link validity tests in `mdox` may fail because upstream documentation links don't exist in the fork. This is expected and can be ignored.
- The release script runs `go mod tidy` which downloads many dependencies — expect this step to be slow on first run.
