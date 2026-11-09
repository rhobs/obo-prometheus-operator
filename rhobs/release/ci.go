// Copyright The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var upstreamWorkflows = []string{
	"checks.yaml", "unit.yaml", "changed-files.yaml", "e2e.yaml",
	"e2e-feature-gated.yaml", "test-prom-version-upgrade.yaml",
	"e2e-prometheus-v2.yaml", "actionlint.yml",
}

func ciHeader(commit string) string {
	return "# Generated from openshift/prometheus-operator@" + commit + ".\n" +
		"# Downstream changes: rhobs/ci.patch; refresh with rhobs/release.sh sync-ci.\n\n"
}

// OpenShift removes these files after importing upstream. Use the closest
// first-parent snapshot that still contains them, without fetching another repo.
func (a *app) ciCommit(source string) (string, error) {
	if a.gitOK("cat-file", "-e", source+":.github/workflows/checks.yaml") {
		return source, nil
	}
	commits, err := a.git("log", "--first-parent", "--format=%H", source, "--", ".github/workflows/checks.yaml")
	if err != nil {
		return "", err
	}
	for _, commit := range strings.Fields(commits) {
		for _, candidate := range []string{commit, commit + "^"} {
			if a.gitOK("cat-file", "-e", candidate+":.github/workflows/checks.yaml") {
				resolved, err := a.git("rev-parse", candidate+"^{commit}")
				return strings.TrimSpace(resolved), err
			}
		}
	}
	return "", fmt.Errorf("no upstream CI found in source history; fetch the complete source branch history")
}

func (a *app) syncCI(source, destination string) (string, error) {
	commit, err := a.ciCommit(source)
	if err != nil {
		return "", err
	}
	directory, err := os.MkdirTemp(temporaryParent(), "rhobs-upstream-ci-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(directory)
	for _, file := range upstreamWorkflows {
		path := ".github/workflows/" + file
		contents, err := a.gitShow(commit, path)
		if err != nil {
			return "", fmt.Errorf("read upstream CI %s: %w", path, err)
		}
		if err := writeFile(filepath.Join(directory, path), []byte(contents)); err != nil {
			return "", err
		}
	}
	patch := filepath.Join(a.Root, "rhobs/ci.patch")
	if _, err := a.run(directory, "git", "apply", "--check", patch); err != nil {
		return "", fmt.Errorf("upstream CI changed; refresh the small downstream delta in rhobs/ci.patch: %w", err)
	}
	if _, err := a.run(directory, "git", "apply", patch); err != nil {
		return "", err
	}
	// Prefer the current source's unmodified shared action, even when the
	// workflow definitions had to be recovered from an older source snapshot.
	const action = ".github/actions/setup-e2e/action.yaml"
	actionCommit := source
	if !a.gitOK("cat-file", "-e", actionCommit+":"+action) {
		actionCommit = commit
	}
	setup, err := a.gitShow(actionCommit, action)
	if err != nil {
		return "", err
	}
	// Do not touch destination files until every upstream file and patch validates.
	for _, file := range upstreamWorkflows {
		path := ".github/workflows/" + file
		contents, err := os.ReadFile(filepath.Join(directory, path))
		if err != nil {
			return "", err
		}
		if err := writeFile(filepath.Join(destination, path), append([]byte(ciHeader(commit)), contents...)); err != nil {
			return "", err
		}
	}
	if err := writeFile(filepath.Join(destination, action), []byte(setup)); err != nil {
		return "", err
	}
	return commit, nil
}

// Overlay individual managed files in the fresh source worktree. Unlike merging
// unrelated histories, this also works when source-owned CI files already exist.
func (a *app) applyOverlay(commit string) error {
	files, err := a.git("ls-tree", "-r", "-z", "--name-only", commit)
	if err != nil {
		return err
	}
	paths := strings.Split(strings.TrimSuffix(files, "\x00"), "\x00")
	args := append([]string{"restore", "--source=" + commit, "--staged", "--worktree", "--"}, paths...)
	_, err = a.git(args...)
	return err
}
