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

func (a *app) publishTags(remote string) error {
	data, err := a.verifyRelease()
	if err != nil {
		return err
	}
	if err := a.assertClean(); err != nil {
		return err
	}
	commit, err := a.git("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	commit = strings.TrimSpace(commit)
	refs, err := a.remoteRefs(remote, "tags")
	if err != nil {
		return err
	}
	refspecs := []string{"push", "--atomic", remote}
	for _, tag := range releaseTags(data.Version) {
		if _, exists := refs["refs/tags/"+tag]; exists {
			existing, err := a.fetchRef(remote, "refs/tags/"+tag, "refs/rhobs/published/"+tag)
			if err != nil {
				return err
			}
			if existing != commit {
				return fmt.Errorf("published tag %s points at another commit; refusing to overwrite it", tag)
			}
			refspecs = append(refspecs, "refs/rhobs/published/"+tag+":refs/tags/"+tag)
			continue
		}
		if a.gitOK("rev-parse", "--verify", "refs/tags/"+tag) {
			existing, err := a.git("rev-parse", "refs/tags/"+tag+"^{commit}")
			if err != nil {
				return err
			}
			if strings.TrimSpace(existing) != commit {
				return fmt.Errorf("local tag %s points at another commit", tag)
			}
		} else if _, err := a.git("tag", "-a", tag, "-m", "v"+data.Version, commit); err != nil {
			return err
		}
		refspecs = append(refspecs, "refs/tags/"+tag+":refs/tags/"+tag)
	}
	_, err = a.git(refspecs...)
	return err
}

func (a *app) checkImport(published bool) error {
	directory, err := os.MkdirTemp(temporaryParent(), "rhobs-import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	importDir := filepath.Join(a.Root, "rhobs/test/import")
	for _, filename := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(importDir, filename))
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(directory, filename), data); err != nil {
			return err
		}
	}
	modfile := "-modfile=" + filepath.Join(directory, "go.mod")
	paths := []string{a.Root, filepath.Join(a.Root, "pkg/apis/monitoring"), filepath.Join(a.Root, "pkg/client")}
	env := map[string]string{"GOWORK": "off"}
	for i, module := range selfModules {
		args := []string{"go", "mod", "edit", modfile, "-replace", module + "=" + paths[i]}
		if published {
			args = []string{"go", "mod", "edit", modfile, "-dropreplace", module}
		}
		if _, err := a.execute(command{Dir: importDir, Env: env, Args: args}); err != nil {
			return err
		}
	}
	output, err := a.execute(command{Dir: importDir, Env: env, Args: []string{"go", "test", "-mod=mod", modfile, "./..."}})
	if err == nil {
		_, err = fmt.Fprint(a.Out, output)
	}
	return err
}
