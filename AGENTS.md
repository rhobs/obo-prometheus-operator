# RHOBS release overlay

On `rhobs-scripts`, this checkout contains release tooling, not operator source.
Generated release branches contain both the operator and this overlay. See
`rhobs/README.md` for the complete process.

- Source new release series from the highest numerical `release-x.y` branch in
  `openshift/prometheus-operator`, pinned to an immutable commit. Never substitute
  a Prometheus Operator upstream tag.
- Rebuild using the previous release's `rhobs/release.json`. Keep its source
  branch and operator base version; increment the numeric `-rhobsN` revision.
- Keep `origin` as the personal fork and `upstream` as canonical RHOBS. The tools
  accept repository URLs and do not require remote renaming or token-bearing URLs.
- Planning is read-only apart from fetching private Git refs. Preparing a local
  worktree requires `--execute`; pushing/opening a PR also requires `--publish-pr`.
  Do not execute either or publish artifacts without explicit authorization.
- Do not discard existing work or force-push a release branch/tag. Use isolated
  worktrees. Preserve and check OpenShift's third-party vendor patches.
- Reuse upstream validation workflows and the shared E2E action. Keep downstream
  edits in `rhobs/ci.patch`; regenerate with `rhobs/release.sh sync-ci` rather than
  hand-maintaining parallel CI matrices. Skip documentation/link and spelling
  checks, including golangci `misspell`.
- Keep release tooling in the standalone, standard-library-only Go module at
  `rhobs/release/`. Use `bash rhobs/release.sh` to avoid the operator's module and
  vendor settings, including for workflow helper commands.
- Test overlay changes with `bash rhobs/ci.sh tools` and `actionlint`. The operator
  Makefile/tests only exist after applying the overlay to a source snapshot.
- Default-branch installation of the preparation controller is a separate change
  from updating `rhobs-scripts`; do not merge all operator/overlay files into the
  lightweight `rhobs-main` branch.
