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
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (a *app) transform(data Metadata) error {
	if err := validateMetadata(data); err != nil {
		return err
	}
	if err := a.assertClean(); err != nil {
		return err
	}
	if !a.gitOK("merge-base", "--is-ancestor", data.SourceCommit, "HEAD") {
		return fmt.Errorf("release checkout is not based on the pinned source commit")
	}
	expected, err := a.git("rev-parse", data.ScriptsCommit+":rhobs")
	if err != nil {
		return err
	}
	actual, err := a.git("rev-parse", "HEAD:rhobs")
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("RHOBS overlay does not match scripts_commit")
	}
	version, err := readText(filepath.Join(a.Root, "VERSION"))
	if err != nil {
		return err
	}
	if strings.TrimSpace(version) != data.SourceVersion {
		return fmt.Errorf("source VERSION no longer matches the release plan")
	}
	if err := a.checkSourceSelfVendor(data.SourceCommit); err != nil {
		return err
	}
	output, err := a.git("ls-files", "-z")
	if err != nil {
		return err
	}
	paths := strings.Split(strings.TrimSuffix(output, "\x00"), "\x00")
	category := regexp.MustCompile(`\+kubebuilder:resource:categories="prometheus-operator"[^\n]*`)
	for _, name := range paths {
		if strings.HasPrefix(name, "vendor/") || strings.HasPrefix(name, "rhobs/") {
			continue
		}
		path := filepath.Join(a.Root, filepath.FromSlash(name))
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !utf8.Valid(content) {
			continue
		}
		text := string(content)
		updated := strings.ReplaceAll(text, "monitoring.coreos.com", "monitoring.rhobs")
		if !strings.HasPrefix(name, "Documentation/") {
			updated = strings.ReplaceAll(updated, upstreamModule, modulePath)
			updated = strings.ReplaceAll(updated, "quay.io/prometheus-operator/", "quay.io/rhobs/obo-")
			updated = category.ReplaceAllString(updated, `+kubebuilder:resource:categories="rhobs-prometheus-operator"`)
			if strings.HasPrefix(name, "test/") {
				for _, image := range []string{"prometheus-alertmanager-test-webhook", "instrumented-sample-app"} {
					updated = strings.ReplaceAll(updated, "quay.io/rhobs/obo-"+image, "quay.io/prometheus-operator/"+image)
				}
			}
		}
		if updated != text {
			if err := writeFile(path, []byte(updated)); err != nil {
				return err
			}
		}
	}
	for _, directory := range []string{"example/prometheus-operator-crd", "example/prometheus-operator-crd-full"} {
		matches, err := filepath.Glob(filepath.Join(a.Root, directory, "monitoring.coreos.com*"))
		if err != nil {
			return err
		}
		for _, path := range matches {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	mdox := filepath.Join(a.Root, ".mdox.validate.yaml")
	text, err := readText(mdox)
	if err == nil {
		if err := writeFile(mdox, []byte(strings.ReplaceAll(text, `monitoring\.coreos\.com`, `monitoring\.rhobs`))); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := a.updateSelfRequirements(paths, data.Version); err != nil {
		return err
	}
	golangci := filepath.Join(a.Root, ".golangci.yml")
	text, err = readText(golangci)
	if err != nil {
		return err
	}
	misspell := regexp.MustCompile(`(?m)^[\t ]*-[\t ]+misspell[\t ]*(?:#[^\n]*)?\r?\n`)
	if err := writeFile(golangci, []byte(misspell.ReplaceAllString(text, ""))); err != nil {
		return err
	}
	makefile := filepath.Join(a.Root, "Makefile")
	text, err = readText(makefile)
	if err != nil {
		return err
	}
	text = strings.ReplaceAll(text, "gofmt -s -w .", "git ls-files -z '*.go' ':(exclude)vendor/**' | xargs -0 -r gofmt -s -w")
	find := regexp.MustCompile(`\tfind \. -name \*\.jsonnet[^\n]+`)
	text = find.ReplaceAllStringFunc(text, func(string) string {
		return "\tfind . -type f \\( -name '*.jsonnet' -o -name '*.libsonnet' \\) -not -path '*/vendor/*' -print0 | xargs -0 -r $(JSONNETFMT_BINARY) -i"
	})
	text = strings.ReplaceAll(text, "$(CONTAINER_CLI) build ", "$(CONTAINER_CLI) build --label org.opencontainers.image.revision=$(BUILD_REVISION) --label org.opencontainers.image.version=$(VERSION) ")
	// Preserve upstream's standard CI recipe (make format generate), but remove
	// only the known-broken documentation/link checks from its prerequisites.
	for target, prerequisite := range map[string]string{"format": "docs", "generate": "generate-docs"} {
		pattern := regexp.MustCompile(`(?m)^` + target + `:[^\n]*`)
		text = pattern.ReplaceAllStringFunc(text, func(line string) string {
			return regexp.MustCompile(`\s+`+prerequisite+`\b`).ReplaceAllString(line, "")
		})
	}
	if err := writeFile(makefile, []byte(text)); err != nil {
		return err
	}
	if err := a.updateUpgradeURLs(data.PreviousVersion); err != nil {
		return err
	}
	// Keep upstream validation; exclude only publication, housekeeping, and spelling.
	for _, file := range []string{"release.yaml", "publish.yaml", "stale.yaml", "automerge-dependabot.yaml", "spell-check.yaml", "cspell.json"} {
		if err := os.Remove(filepath.Join(a.Root, ".github/workflows", file)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	data.CICommit, err = a.syncCI(data.SourceCommit, a.Root)
	if err != nil {
		return err
	}
	// The unchanged upstream E2E setup imports this file before building images.
	// Match the reloader/webhook defaults to the images loaded into Kind.
	envPath := filepath.Join(a.Root, ".github/env")
	environment, err := readText(envPath)
	if err != nil {
		return err
	}
	environment = regexp.MustCompile(`(?m)^TAG=[^\n]*\n?`).ReplaceAllString(environment, "")
	if err := writeFile(envPath, []byte(strings.TrimRight(environment, "\n")+"\nTAG=v"+data.Version+"\n")); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(a.Root, "VERSION"), []byte(data.Version+"\n")); err != nil {
		return err
	}
	data.Changed = nil
	var metadata bytes.Buffer
	if err := writeJSON(&metadata, data); err != nil {
		return err
	}
	return writeFile(filepath.Join(a.Root, metadataPath), metadata.Bytes())
}

func (a *app) updateSelfRequirements(paths []string, version string) error {
	for _, path := range paths {
		if strings.HasPrefix(path, "vendor/") || strings.HasPrefix(path, "rhobs/") || path != "go.mod" && !strings.HasSuffix(path, "/go.mod") {
			continue
		}
		dir := filepath.Dir(filepath.Join(a.Root, path))
		output, err := a.run(dir, "go", "mod", "edit", "-json")
		if err != nil {
			return err
		}
		var module struct {
			Require []struct{ Path string }
		}
		if err := json.Unmarshal([]byte(output), &module); err != nil {
			return err
		}
		for _, required := range module.Require {
			for _, self := range selfModules {
				if required.Path == self {
					if _, err := a.run(dir, "go", "mod", "edit", "-require", self+"@v"+version); err != nil {
						return err
					}
				}
			}
		}
	}
	for _, module := range selfModules {
		if _, err := a.run(filepath.Join(a.Root, "rhobs/test/import"), "go", "mod", "edit", "-require", module+"@v"+version); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) updateUpgradeURLs(previous string) error {
	path := filepath.Join(a.Root, "test/e2e/main_test.go")
	text, err := readText(path)
	if err != nil {
		return err
	}
	base := "https://raw.githubusercontent.com/rhobs/obo-prometheus-operator/v" + previous
	for _, entry := range [][2]string{{"prevStableVersionURL", "/VERSION"}, {"prevExampleDir", "/example"}, {"prevResourcesDir", "/test/framework/resources"}} {
		pattern := regexp.MustCompile(`(?m)^([\t ]*)` + entry[0] + ` := [^\n]*`)
		matches := pattern.FindAllStringSubmatch(text, -1)
		if len(matches) != 1 {
			return fmt.Errorf("source upgrade test changed: could not replace %s", entry[0])
		}
		replacement := matches[0][1] + entry[0] + " := " + strconv.Quote(base+entry[1])
		text = pattern.ReplaceAllStringFunc(text, func(string) string { return replacement })
	}
	old := regexp.MustCompile(`(?m)^([\t ]*)(prometheusOperatorGithubBranchURL := [^\n]*)`)
	text = old.ReplaceAllString(text, "${1}// ${2}")
	return writeFile(path, []byte(text))
}
