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
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLatestBranch(t *testing.T) {
	refs := make(map[string]string)
	for _, branch := range []string{"release-0.99", "release-4.99", "release-5.9", "release-5.10", "release-99.1-rc", "main"} {
		refs["refs/heads/"+branch] = "sha"
	}
	actual, err := latestBranch(refs)
	if err != nil {
		t.Fatal(err)
	}
	requireEqual(t, actual, "release-5.10")
	_, err = latestBranch(map[string]string{"refs/heads/main": "sha"})
	requireError(t, err, "no release-x.y branches")
}

func TestNextVersion(t *testing.T) {
	refs := make(map[string]string)
	for _, tag := range []string{"v0.94.2-rhobs1", "v0.94.2-rhobs9", "pkg/client/v0.94.2-rhobs99", "v0.94.2", "v0.94.20-rhobs77"} {
		refs["refs/tags/"+tag] = "sha"
	}
	actual, err := nextVersion("0.94.2", refs)
	if err != nil {
		t.Fatal(err)
	}
	requireEqual(t, actual, "0.94.2-rhobs10")
	_, err = nextVersion("0.94.2", map[string]string{"refs/tags/pkg/client/v0.94.2-rhobs1": "sha"})
	requireError(t, err, "partial publication")
	_, err = nextVersion("0.94.2", map[string]string{fmt.Sprintf("refs/tags/v0.94.2-rhobs%d", uint64(math.MaxUint64)): "sha"})
	requireError(t, err, "overflow")
}

func TestVersionKeys(t *testing.T) {
	a, err := versionKey("v0.94.2-rhobs10")
	if err != nil {
		t.Fatal(err)
	}
	b, err := versionKey("v0.94.2-rhobs9")
	if err != nil {
		t.Fatal(err)
	}
	requireEqual(t, compareVersion(a, b), 1)
	for _, version := range []string{"0.94.2", "0.94.2-rhobs0", "0.94.2-rhobs1extra", "../0.94.2-rhobs1", "0.094.2-rhobs1", "18446744073709551616.1.0-rhobs1"} {
		t.Run(version, func(t *testing.T) {
			_, err := normalizeVersion(version)
			requireError(t, err, "invalid downstream version")
		})
	}
}

func TestGitHubRepository(t *testing.T) {
	for _, value := range []string{"rhobs/obo-prometheus-operator", "https://github.com/rhobs/obo-prometheus-operator.git", "git@github.com:rhobs/obo-prometheus-operator.git"} {
		actual, err := githubRepository(value)
		if err != nil {
			t.Fatal(err)
		}
		requireEqual(t, actual, "rhobs/obo-prometheus-operator")
	}
}

func TestInitialReleaseUsesUntaggedHighestBranch(t *testing.T) {
	f := newFixture(t)
	data := f.plan()
	requireEqual(t, data.SourceBranch, "release-5.2")
	requireEqual(t, data.SourceCommit, f.SourceCommit)
	requireEqual(t, data.Version, "0.94.2-rhobs1")
	requireEqual(t, data.PreviousVersion, "0.93.1-rhobs1")
	requireEqual(t, *data.Changed, true)
	requireEqual(t, f.git(f.Source, "tag", "--list"), "")
	requireEqual(t, f.git(f.Controller, "tag", "--list"), "")
}

func TestRebuildKeepsSourceBranch(t *testing.T) {
	f := newFixture(t)
	f.published()
	f.write(f.Source, "fix.txt", "OpenShift backport\n")
	fixed := f.commit(f.Source, "backport fix")
	f.git(f.Source, "branch", "release-6.0")
	options := f.options()
	options.Mode, options.Source = "rebuild", ""
	data := f.plan(options)
	requireEqual(t, data.SourceBranch, "release-5.2")
	requireEqual(t, data.SourceCommit, fixed)
	requireEqual(t, data.Version, "0.94.2-rhobs2")
	requireEqual(t, *data.Changed, true)
}

func TestUnchangedSourceAndForcedRebuild(t *testing.T) {
	f := newFixture(t)
	f.published()
	options := f.options()
	options.Mode = "rebuild"
	requireEqual(t, *f.plan(options).Changed, false)
	options.Force = true
	requireEqual(t, *f.plan(options).Changed, true)
}

func TestNewSeriesWithSameOperatorVersionDoesNotReuseTags(t *testing.T) {
	f := newFixture(t)
	f.published()
	f.git(f.Source, "branch", "release-5.3")
	data := f.plan()
	requireEqual(t, data.SourceBranch, "release-5.3")
	requireEqual(t, data.Version, "0.94.2-rhobs2")
}

func TestExactSourceSnapshot(t *testing.T) {
	f := newFixture(t)
	f.write(f.Source, "fix.txt", "later change\n")
	f.commit(f.Source, "later change")
	options := f.options()
	options.SourceCommit = f.SourceCommit
	requireEqual(t, f.plan(options).SourceCommit, f.SourceCommit)
}

func TestRebuildSafety(t *testing.T) {
	tests := []struct {
		name, error string
		setup       func(*fixture, *planOptions)
	}{
		{"legacy release", "legacy releases", func(f *fixture, _ *planOptions) {}},
		{"version rollover", "VERSION changed", func(f *fixture, _ *planOptions) {
			f.published()
			f.write(f.Source, "VERSION", "0.95.0\n")
			f.commit(f.Source, "operator version rollover")
		}},
		{"rewritten history", "history was rewritten", func(f *fixture, _ *planOptions) {
			f.published()
			f.git(f.Source, "checkout", "--orphan", "rewritten")
			f.commit(f.Source, "unrelated history")
			f.git(f.Source, "branch", "-f", "release-5.2", "HEAD")
		}},
		{"missing branch", "couldn't find remote ref", func(f *fixture, _ *planOptions) {
			f.published()
			f.git(f.Source, "checkout", "main")
			f.git(f.Source, "branch", "-D", "release-5.2")
		}},
		{"changed repository", "cannot change", func(f *fixture, options *planOptions) {
			f.published()
			options.Source = f.Downstream
		}},
		{"stale previous revision", "latest published revision", func(f *fixture, options *planOptions) {
			f.published()
			forced := f.options()
			forced.Mode, forced.Force = "rebuild", true
			f.published(f.plan(forced))
			options.PreviousVersion = "0.94.2-rhobs1"
		}},
		{"partial previous publication", "partial tag publication", func(f *fixture, _ *planOptions) {
			f.published()
			f.git(f.Downstream, "tag", "-d", "pkg/client/v0.94.2-rhobs1")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			options := f.options()
			options.Mode = "rebuild"
			test.setup(f, &options)
			_, err := f.App.planRelease(options)
			requireError(t, err, test.error)
		})
	}
}

func TestTransformPreservesVendorAndDocumentationLinks(t *testing.T) {
	f := newFixture(t)
	data := f.plan()
	a := f.checkout(data)
	before := f.read(a.Root, "vendor/example.org/dependency/patch.go")
	if err := a.transform(data); err != nil {
		t.Fatal(err)
	}
	requireEqual(t, f.read(a.Root, "vendor/example.org/dependency/patch.go"), before)
	for _, check := range [][2]string{
		{"pkg/example.go", "monitoring.rhobs"},
		{"Documentation/example.md", "monitoring.rhobs"},
		{"Documentation/example.md", "https://github.com/prometheus-operator/prometheus-operator/issues/1"},
		{".golangci.yml", "revive"},
		{"Makefile", ":(exclude)vendor/**"},
		{"Makefile", "org.opencontainers.image.revision"},
		{"Makefile", "-type f \\( -name"},
		{"Makefile", "format: go-fmt jsonnet-fmt fix-license shellcheck\n"},
		{"Makefile", "generate: k8s-gen generate-metrics image-builder-version\n"},
	} {
		if !strings.Contains(f.read(a.Root, check[0]), check[1]) {
			t.Errorf("%s does not contain %q", check[0], check[1])
		}
	}
	if strings.Contains(f.read(a.Root, "pkg/example.go"), "shortName") || strings.Contains(f.read(a.Root, ".golangci.yml"), "misspell") {
		t.Fatal("upstream short names or spelling checks remain")
	}
	for _, path := range []string{".github/workflows/spell-check.yaml", "example/prometheus-operator-crd/monitoring.coreos.com_prometheuses.yaml"} {
		if _, err := os.Stat(filepath.Join(a.Root, path)); !os.IsNotExist(err) {
			t.Errorf("inherited file was not removed: %s", path)
		}
	}
	if _, err := os.Stat(filepath.Join(a.Root, ".github/workflows/rhobs-checks.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.Root, ".github/workflows/checks.yaml")); err != nil {
		t.Fatal("upstream checks were not restored:", err)
	}
}

func TestTransformUpdatesSelfRequirementsByIdentity(t *testing.T) {
	f := newFixture(t)
	a := f.prepared(f.plan())
	for _, path := range []string{"go.mod", "pkg/client/go.mod", "rhobs/test/import/go.mod"} {
		text := f.read(a.Root, path)
		if !strings.Contains(text, "v0.94.2-rhobs1") || strings.Contains(text, "v0.94.1") {
			t.Errorf("incorrect self-module requirement in %s", path)
		}
	}
	if !strings.Contains(f.read(a.Root, "go.mod"), "=> ./pkg/client") || !strings.Contains(f.read(a.Root, "test/e2e/main_test.go"), "/v0.93.1-rhobs1/VERSION") {
		t.Fatal("local replacements or immutable upgrade URLs were not retained")
	}
	if _, err := a.verifyRelease(); err != nil {
		t.Fatal(err)
	}
	data, err := loadMetadata(filepath.Join(a.Root, metadataPath))
	if err != nil {
		t.Fatal(err)
	}
	if data.Changed != nil {
		t.Fatal("planning-only changed field was persisted")
	}
	requireEqual(t, data.CICommit, f.CICommit)
	if !strings.Contains(f.read(a.Root, ".github/env"), "TAG=v0.94.2-rhobs1") {
		t.Fatal("upstream E2E setup would not load matching version-tagged images")
	}
}

func TestTransformRejectsDirtyCheckout(t *testing.T) {
	f := newFixture(t)
	data := f.plan()
	a := f.checkout(data)
	f.write(a.Root, "user-work.txt", "leave me alone\n")
	requireError(t, a.transform(data), "not clean")
	requireEqual(t, f.read(a.Root, "user-work.txt"), "leave me alone\n")
}

func TestVendorPreservation(t *testing.T) {
	t.Run("external patch", func(t *testing.T) {
		f := newFixture(t)
		data := f.plan()
		a := f.prepared(data)
		f.write(a.Root, "vendor/example.org/dependency/patch.go", "package dependency\n")
		requireError(t, a.checkVendor(data), "third-party vendor")
	})
	t.Run("self-module vendor-only patch", func(t *testing.T) {
		f := newFixture(t)
		f.write(f.Source, "vendor/"+upstreamModule+"/pkg/client/client.go", "package client\n// vendor-only fix\n")
		f.commit(f.Source, "patch only the vendored client")
		data := f.plan()
		a := f.checkout(data)
		requireError(t, a.transform(data), "vendor-only patch")
		if err := a.assertClean(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestMetadataMismatch(t *testing.T) {
	f := newFixture(t)
	a := f.prepared(f.plan())
	f.write(a.Root, "VERSION", "0.94.2-rhobs9\n")
	_, err := a.verifyRelease()
	requireError(t, err, "VERSION and release metadata")
}

func TestPrepareDryRun(t *testing.T) {
	f := newFixture(t)
	if err := f.App.prepareRelease(prepareOptions{}, f.plan()); err != nil {
		t.Fatal(err)
	}
	if err := f.App.assertClean(); err != nil {
		t.Fatal(err)
	}
	requireEqual(t, f.git(f.Controller, "branch", "--list", "rhobs-pr-*"), "")
	if strings.Contains(f.git(f.Downstream, "branch", "--list"), "rhobs-rel-") {
		t.Fatal("dry run created a downstream branch")
	}
}

func TestPrepareReusesExistingPR(t *testing.T) {
	f := newFixture(t)
	data := f.plan()
	original := f.App.Runner
	f.App.Runner = func(c command) (commandResult, error) {
		if c.Args[0] == "gh" {
			if strings.Join(c.Args[:4], " ") != "gh api --method GET" || !strings.Contains(strings.Join(c.Args, " "), "head=contributor:rhobs-pr-0.94.2-rhobs1") {
				t.Fatalf("incorrect PR lookup: %v", c.Args)
			}
			return commandResult{Stdout: `[{"html_url":"https://example.invalid/pr/1"}]`}, nil
		}
		return original(c)
	}
	options := prepareOptions{planOptions: defaultPlanOptions(), Execute: true, PublishPR: true, ForkRepository: "contributor/obo-prometheus-operator"}
	if err := f.App.prepareRelease(options, data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.Out.String(), "https://example.invalid/pr/1") {
		t.Fatal("existing PR was not reused")
	}
	requireEqual(t, f.git(f.Controller, "branch", "--list", "rhobs-pr-*"), "")
}

func TestCLIUsesPinnedPlanAndRejectsStalePlan(t *testing.T) {
	f := newFixture(t)
	data := f.plan()
	file, err := os.Create(filepath.Join(f.Controller, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(file, data); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	// Put the plan outside the checkout before later cleanliness checks.
	planPath := filepath.Join(f.Dir, "plan.json")
	if err := os.Rename(file.Name(), planPath); err != nil {
		t.Fatal(err)
	}
	f.write(f.Source, "fix.txt", "new tip after planning\n")
	f.commit(f.Source, "new fix")
	var output bytes.Buffer
	args := []string{"--root", f.Controller, "prepare", "--plan", planPath, "--downstream", f.Downstream}
	if err := runCLI(args, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	var actual Metadata
	if err := json.Unmarshal(output.Bytes(), &actual); err != nil {
		t.Fatal(err)
	}
	requireEqual(t, actual.SourceCommit, data.SourceCommit)
	f.published(data)
	requireError(t, runCLI(args, io.Discard, io.Discard), "stale")
}

func TestCLIRequiresExplicitExecutionToPublish(t *testing.T) {
	f := newFixture(t)
	requireError(t, runCLI([]string{"--root", f.Controller, "prepare", "--mode", "new", "--publish-pr"}, io.Discard, io.Discard), "requires --execute")
}

func TestTagPublicationAndRetry(t *testing.T) {
	f := newFixture(t)
	data := f.plan()
	a := f.prepared(data)
	f.commit(a.Root, "prepared release")
	f.git(a.Root, "tag", "v99.0.0")
	for i := 0; i < 2; i++ {
		if err := a.publishTags(f.Downstream); err != nil {
			t.Fatal(err)
		}
	}
	tags := "\n" + f.git(f.Downstream, "tag", "--list") + "\n"
	if strings.Contains(tags, "\nv99.0.0\n") {
		t.Fatal("an unrelated tag was published")
	}
	for _, tag := range releaseTags(data.Version) {
		if !strings.Contains(tags, "\n"+tag+"\n") {
			t.Errorf("missing published tag %s", tag)
		}
	}
}

func TestPartialTagPublication(t *testing.T) {
	f := newFixture(t)
	data := f.plan()
	a := f.prepared(data)
	f.commit(a.Root, "prepared release")
	tag := releaseTags(data.Version)[0]
	f.git(a.Root, "tag", "-a", tag, "-m", "already published")
	f.git(a.Root, "push", f.Downstream, "refs/tags/"+tag)
	if err := a.publishTags(f.Downstream); err != nil {
		t.Fatal(err)
	}
	requireEqual(t, len(strings.Split(f.git(f.Downstream, "tag", "--list", "*0.94.2-rhobs1"), "\n")), 3)
}

func TestTagCollisionCannotOverwriteRelease(t *testing.T) {
	f := newFixture(t)
	a := f.prepared(f.plan())
	f.commit(a.Root, "prepared release")
	f.git(f.Downstream, "tag", "v0.94.2-rhobs1")
	requireError(t, a.publishTags(f.Downstream), "another commit")
	requireEqual(t, f.git(f.Downstream, "rev-parse", "v0.94.2-rhobs1"), f.Legacy)
}

func TestPublishedImportsIgnoreLocalWorkspaceAndReplacements(t *testing.T) {
	root := projectRoot(t)
	original, err := os.ReadFile(filepath.Join(root, "rhobs/test/import/go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(root, io.Discard, io.Discard)
	var commands []command
	a.Runner = func(c command) (commandResult, error) {
		commands = append(commands, c)
		return commandResult{}, nil
	}
	if err := a.checkImport(true); err != nil {
		t.Fatal(err)
	}
	requireEqual(t, len(commands), 4)
	for i, c := range commands {
		requireEqual(t, c.Env["GOWORK"], "off")
		if i < 3 {
			requireEqual(t, c.Args[len(c.Args)-2], "-dropreplace")
			requireEqual(t, c.Args[len(c.Args)-1], selfModules[i])
		}
	}
	actual, err := os.ReadFile(filepath.Join(root, "rhobs/test/import/go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	requireEqual(t, string(actual), string(original))
}
