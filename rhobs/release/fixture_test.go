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
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("RHOBS_TEST_HELPER") == "1" {
		os.Exit(helperProcess())
	}
	if os.Getenv("TMPDIR") == "" {
		if info, err := os.Stat("/tmp/opencode"); err == nil && info.IsDir() {
			_ = os.Setenv("TMPDIR", "/tmp/opencode")
		}
	}
	os.Exit(m.Run())
}

// Re-execute the Go test binary as the CLI or fake Docker/Make. These helpers
// never contact registries and keep publication tests independent of Docker.
func helperProcess() int {
	name := filepath.Base(os.Args[0])
	if name == "rhobs-release" {
		if err := runCLI(os.Args[1:], os.Stdout, os.Stderr); err != nil && !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "Release error:", err)
			return 1
		}
		return 0
	}
	log, err := os.OpenFile(os.Getenv("FAKE_LOG"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	err = json.NewEncoder(log).Encode(append([]string{name}, os.Args[1:]...))
	closeErr := log.Close()
	if err != nil || closeErr != nil {
		return 1
	}
	if name == "make" {
		return 0
	}
	if name != "docker" {
		return 1
	}
	contents, err := os.ReadFile(os.Getenv("FAKE_REGISTRY"))
	if err != nil {
		return 1
	}
	var state map[string]string
	if err := json.Unmarshal(contents, &state); err != nil {
		return 1
	}
	args := os.Args[1:]
	if len(args) >= 3 && reflect.DeepEqual(args[:3], []string{"buildx", "imagetools", "inspect"}) {
		if os.Getenv("FAKE_REGISTRY_ERROR") == "true" {
			fmt.Fprintln(os.Stderr, "denied: authentication required")
			return 1
		}
		if _, exists := state[args[len(args)-1]]; !exists {
			fmt.Fprintln(os.Stderr, "manifest unknown")
			return 1
		}
	} else if len(args) >= 2 && reflect.DeepEqual(args[:2], []string{"image", "inspect"}) {
		fmt.Fprintln(os.Stdout, state[args[len(args)-1]])
	} else if len(args) >= 3 && reflect.DeepEqual(args[:3], []string{"buildx", "imagetools", "create"}) {
		for i, arg := range args {
			if arg == "--tag" && i+1 < len(args) {
				state[args[i+1]] = os.Getenv("FAKE_COMMIT")
			}
		}
		contents, err := json.Marshal(state)
		if err != nil || os.WriteFile(os.Getenv("FAKE_REGISTRY"), contents, 0o600) != nil {
			return 1
		}
	}
	return 0
}

type fixture struct {
	t                                             *testing.T
	Dir, Source, Downstream, Controller           string
	SourceCommit, ScriptsCommit, Legacy, CICommit string
	App                                           *app
	Out, Stderr                                   bytes.Buffer
}

func projectRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate release tool sources")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	t.Setenv("GITHUB_REF", "")
	f := &fixture{t: t, Dir: t.TempDir()}
	f.Source = f.initRepo("source")
	f.Downstream = f.initRepo("downstream")
	f.Controller = f.initRepo("scripts")
	f.App = newApp(f.Controller, &f.Out, &f.Stderr)
	f.write(f.Source, "VERSION", "0.94.2\n")
	f.write(f.Source, "go.mod", `module github.com/prometheus-operator/prometheus-operator
go 1.20
require (
    github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.94.1
    github.com/prometheus-operator/prometheus-operator/pkg/client v0.94.1
)
replace github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring => ./pkg/apis/monitoring
replace github.com/prometheus-operator/prometheus-operator/pkg/client => ./pkg/client
`)
	f.write(f.Source, "pkg/apis/monitoring/go.mod", "module "+upstreamModule+"/pkg/apis/monitoring\ngo 1.20\n")
	f.write(f.Source, "pkg/client/go.mod", `module github.com/prometheus-operator/prometheus-operator/pkg/client
go 1.20
require github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.94.1
replace github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring => ../apis/monitoring
`)
	f.write(f.Source, "pkg/client/client.go", "package client\n")
	f.write(f.Source, "pkg/example.go", "package example\n// monitoring.coreos.com\n// quay.io/prometheus-operator/prometheus-operator\n// +kubebuilder:resource:categories=\"prometheus-operator\",shortName=\"prom\"\n")
	f.write(f.Source, "Documentation/example.md", "monitoring.coreos.com https://github.com/prometheus-operator/prometheus-operator/issues/1\n")
	f.write(f.Source, "vendor/example.org/dependency/patch.go", "package dependency\n// OpenShift-only fix; monitoring.coreos.com must not be rewritten here\n")
	f.write(f.Source, "vendor/"+upstreamModule+"/pkg/client/client.go", "package client\n")
	f.write(f.Source, "Makefile", "go-fmt:\n\tgofmt -s -w .\njsonnet-fmt:\n\tfind . -name *.jsonnet -or -name *.libsonnet -not -path \"*/vendor/*\" -print0 | xargs -0 $(JSONNETFMT_BINARY) -i\noperator-image:\n\t$(CONTAINER_CLI) build -t operator .\n")
	f.write(f.Source, ".golangci.yml", "version: '2'\nlinters:\n  enable:\n    - misspell\n    - revive\n")
	makefile := f.read(f.Source, "Makefile")
	f.write(f.Source, "Makefile", "format: go-fmt jsonnet-fmt fix-license shellcheck docs\ngenerate: k8s-gen generate-metrics generate-docs image-builder-version\n"+makefile)
	f.write(f.Source, ".github/env", "golang-version=1.26\n")
	f.write(f.Source, ".github/workflows/spell-check.yaml", "name: inherited spelling check\n")
	f.write(f.Source, ".github/workflows/checks.yaml", "name: inherited links check\n")
	f.write(f.Source, "example/prometheus-operator-crd/monitoring.coreos.com_prometheuses.yaml", "old CRD\n")
	f.write(f.Source, "test/e2e/main_test.go", `package e2e
func fixture() {
    prevStableVersionURL := "upstream/VERSION"
    prevExampleDir := "upstream/example"
    prevResourcesDir := "upstream/resources"
}
`)
	for path, contents := range upstreamCIFixture(t) {
		f.write(f.Source, path, contents)
	}
	f.commit(f.Source, "upstream source with validation workflows")
	unit := f.read(f.Source, ".github/workflows/unit.yaml")
	f.write(f.Source, ".github/workflows/unit.yaml", unit+"\n# Upstream updates another workflow after checks.yaml last changed.\n")
	f.CICommit = f.commit(f.Source, "upstream improves another workflow")
	for _, file := range upstreamWorkflows {
		if err := os.Remove(filepath.Join(f.Source, ".github/workflows", file)); err != nil {
			t.Fatal(err)
		}
	}
	f.SourceCommit = f.commit(f.Source, "OpenShift removes unused upstream configs")
	f.git(f.Source, "branch", "release-4.9")
	f.git(f.Source, "branch", "release-5.1")
	f.git(f.Source, "checkout", "-qb", "release-5.2")
	f.write(f.Downstream, "VERSION", "0.93.1-rhobs1\n")
	f.Legacy = f.commit(f.Downstream, "legacy downstream release")
	f.git(f.Downstream, "tag", "-a", "v0.93.1-rhobs1", "-m", "legacy release")
	for _, directory := range []string{"rhobs", ".github"} {
		if err := copyDir(filepath.Join(projectRoot(t), directory), filepath.Join(f.Controller, directory)); err != nil {
			t.Fatal(err)
		}
	}
	f.ScriptsCommit = f.commit(f.Controller, "RHOBS overlay")
	return f
}

func copyDir(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(destination, content, info.Mode().Perm())
	})
}

func (f *fixture) initRepo(name string) string {
	f.t.Helper()
	path := filepath.Join(f.Dir, name)
	if err := os.Mkdir(path, 0o755); err != nil {
		f.t.Fatal(err)
	}
	f.git(path, "init", "-q", "-b", "main")
	f.git(path, "config", "user.name", "Release Test")
	f.git(path, "config", "user.email", "release-test@example.invalid")
	f.git(path, "config", "commit.gpgsign", "false")
	f.git(path, "config", "tag.gpgsign", "false")
	return path
}

func (f *fixture) git(root string, args ...string) string {
	f.t.Helper()
	output, err := newApp(root, io.Discard, io.Discard).git(args...)
	if err != nil {
		f.t.Fatal(err)
	}
	return strings.TrimSpace(output)
}

func (f *fixture) write(root, path, content string) {
	f.t.Helper()
	if err := writeFile(filepath.Join(root, path), []byte(content)); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) read(root, path string) string {
	f.t.Helper()
	text, err := readText(filepath.Join(root, path))
	if err != nil {
		f.t.Fatal(err)
	}
	return text
}

func (f *fixture) commit(root, message string) string {
	f.t.Helper()
	f.git(root, "add", "--all")
	f.git(root, "commit", "-qs", "-m", message, "-m", "Assisted-by: opencode (gpt-6.1-sol)")
	return f.git(root, "rev-parse", "HEAD")
}

func (f *fixture) options() planOptions {
	return planOptions{Mode: "new", Source: f.Source, Downstream: f.Downstream, ScriptsRef: f.ScriptsCommit}
}

func (f *fixture) plan(options ...planOptions) Metadata {
	f.t.Helper()
	selected := f.options()
	if len(options) != 0 {
		selected = options[0]
	}
	data, err := f.App.planRelease(selected)
	if err != nil {
		f.t.Fatal(err)
	}
	return data
}

func (f *fixture) checkout(data Metadata) *app {
	f.t.Helper()
	path := filepath.Join(f.Dir, "worktree-"+data.Version)
	f.git(f.Controller, "worktree", "add", "--detach", path, data.SourceCommit)
	if err := newApp(path, io.Discard, io.Discard).applyOverlay(data.ScriptsCommit); err != nil {
		f.t.Fatal(err)
	}
	f.commit(path, "merge RHOBS overlay")
	return newApp(path, &f.Out, &f.Stderr)
}

func (f *fixture) prepared(data Metadata) *app {
	f.t.Helper()
	a := f.checkout(data)
	if err := a.transform(data); err != nil {
		f.t.Fatal(err)
	}
	return a
}

func (f *fixture) published(plans ...Metadata) (Metadata, *app, string) {
	f.t.Helper()
	var data Metadata
	if len(plans) != 0 {
		data = plans[0]
	} else {
		data = f.plan()
	}
	a := f.prepared(data)
	commit := f.commit(a.Root, "chore(release): v"+data.Version)
	for _, tag := range releaseTags(data.Version) {
		f.git(a.Root, "tag", "-a", tag, "-m", tag)
		f.git(a.Root, "push", f.Downstream, "refs/tags/"+tag)
	}
	return data, a, commit
}

func requireError(t *testing.T, err error, contains string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), contains) {
		t.Fatalf("expected error containing %q, got %v", contains, err)
	}
}

func requireEqual[T comparable](t *testing.T, actual, expected T) {
	t.Helper()
	if actual != expected {
		t.Fatalf("got %v, want %v", actual, expected)
	}
}
