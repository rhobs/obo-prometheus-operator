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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Reverse the checked-in delta to recover the exact upstream input without
// downloading fixtures or maintaining a second copy of upstream definitions.
func upstreamCIFixture(t *testing.T) map[string]string {
	t.Helper()
	directory := t.TempDir()
	root := projectRoot(t)
	for _, file := range upstreamWorkflows {
		path := ".github/workflows/" + file
		contents, err := readText(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		_, contents, ok := strings.Cut(contents, "\n\n")
		if !ok {
			t.Fatal("missing generated CI provenance header:", path)
		}
		if err := writeFile(filepath.Join(directory, path), []byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	a := newApp(directory, io.Discard, io.Discard)
	if _, err := a.run(directory, "git", "apply", "--reverse", filepath.Join(root, "rhobs/ci.patch")); err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string)
	for _, file := range upstreamWorkflows {
		path := ".github/workflows/" + file
		contents, err := readText(filepath.Join(directory, path))
		if err != nil {
			t.Fatal(err)
		}
		files[path] = contents
	}
	const action = ".github/actions/setup-e2e/action.yaml"
	contents, err := readText(filepath.Join(root, action))
	if err != nil {
		t.Fatal(err)
	}
	files[action] = contents
	return files
}

func TestReuseUpstreamCIFromSourceHistory(t *testing.T) {
	f := newFixture(t)
	data := f.plan()
	a := f.prepared(data)
	for _, file := range upstreamWorkflows {
		path := ".github/workflows/" + file
		upstream, err := a.gitShow(f.CICommit, path)
		if err != nil {
			t.Fatal(err)
		}
		generated := f.read(a.Root, path)
		if !strings.HasPrefix(generated, ciHeader(f.CICommit)) {
			t.Fatal("missing upstream provenance:", path)
		}
		if !strings.Contains(upstream, "jobs:") || !strings.Contains(generated, "jobs:") {
			t.Fatal("upstream workflow lost its jobs:", path)
		}
	}
	// The E2E action itself is byte-for-byte upstream, not a parallel implementation.
	const action = ".github/actions/setup-e2e/action.yaml"
	upstream, err := a.gitShow(data.SourceCommit, action)
	if err != nil {
		t.Fatal(err)
	}
	requireEqual(t, f.read(a.Root, action), upstream)
	if strings.Contains(f.read(a.Root, ".github/workflows/checks.yaml"), "check-docs:") {
		t.Fatal("link checks were restored")
	}
	if !strings.Contains(f.read(a.Root, ".github/workflows/e2e.yaml"), "suite: [alertmanager, prometheus, prometheusAllNS, thanosruler, operatorUpgrade]") {
		t.Fatal("the upstream E2E matrix was not reused")
	}
}

func TestReuseSourceCIWhenItHasNotBeenRemoved(t *testing.T) {
	f := newFixture(t)
	for path, contents := range upstreamCIFixture(t) {
		f.write(f.Source, path, contents)
	}
	commit := f.commit(f.Source, "OpenShift retains upstream workflows")
	a := f.prepared(f.plan())
	data, err := loadMetadata(filepath.Join(a.Root, metadataPath))
	if err != nil {
		t.Fatal(err)
	}
	requireEqual(t, data.CICommit, commit)
}

func TestCIChangesFailBeforeOverwritingFiles(t *testing.T) {
	f := newFixture(t)
	for path, contents := range upstreamCIFixture(t) {
		if strings.HasSuffix(path, "checks.yaml") {
			contents = "name: incompatible future CI\non:\n  pull_request:\njobs: {}\n"
		}
		f.write(f.Source, path, contents)
	}
	commit := f.commit(f.Source, "upstream restructures CI")
	f.plan() // Fetch the source commit into the controller's object store.
	destination := t.TempDir()
	_, err := f.App.syncCI(commit, destination)
	requireError(t, err, "refresh the small downstream delta")
	entries, err := os.ReadDir(destination)
	if err != nil {
		t.Fatal(err)
	}
	requireEqual(t, len(entries), 0)
}

func TestUpstreamChangedFilesRunsAllChecksForPublication(t *testing.T) {
	f := newFixture(t)
	workflow := f.read(projectRoot(t), ".github/workflows/changed-files.yaml")
	_, body, ok := strings.Cut(workflow, "      run: |\n")
	if !ok {
		t.Fatal("changed-files script missing")
	}
	body, _, _ = strings.Cut(body, "    - run:")
	lines := strings.Split(strings.TrimSpace(body), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, "        ")
	}
	script := strings.Join(lines, "\n")
	for _, event := range []string{"push", "pull_request"} {
		output := filepath.Join(f.Dir, "github-output-"+event)
		sha := f.git(f.Controller, "rev-parse", "HEAD")
		_, err := f.App.execute(command{
			Dir: f.Controller, Args: []string{"bash", "-eo", "pipefail", "-c", script},
			Env: map[string]string{"GITHUB_EVENT_NAME": event, "GITHUB_OUTPUT": output, "BASE_SHA": sha, "HEAD_SHA": sha},
		})
		if err != nil {
			t.Fatal(err)
		}
		want := "non-markdown-files=\n"
		if event == "push" {
			want = "non-markdown-files=all\n"
		}
		requireEqual(t, f.read(f.Dir, filepath.Base(output)), want)
	}
}
