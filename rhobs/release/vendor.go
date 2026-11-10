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
	"crypto/sha1" // Git blob identity, not a cryptographic security decision.
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func firstPartyVendor(path string) bool {
	return strings.HasPrefix(path, "vendor/"+modulePath+"/") || strings.HasPrefix(path, "vendor/"+upstreamModule+"/")
}

func (a *app) treeFiles(commit string, paths ...string) (map[string]string, error) {
	args := append([]string{"ls-tree", "-r", "-z", commit, "--"}, paths...)
	output, err := a.git(args...)
	if err != nil {
		return nil, err
	}
	files := make(map[string]string)
	for _, entry := range strings.Split(output, "\x00") {
		if entry == "" {
			continue
		}
		info, path, ok := strings.Cut(entry, "\t")
		fields := strings.Fields(info)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("invalid Git tree entry: %q", entry)
		}
		files[path] = fields[2]
	}
	return files, nil
}

func externalVendor(root string) (map[string]string, error) {
	files := make(map[string]string)
	err := filepath.WalkDir(filepath.Join(root, "vendor"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == filepath.Join(root, "vendor") && os.IsNotExist(err) {
				return nil
			}
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() || relative == "vendor/modules.txt" || firstPartyVendor(relative) {
			return nil
		}
		var content []byte
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			content = []byte(target)
		} else {
			content, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		hash := sha1.New()
		fmt.Fprintf(hash, "blob %d\x00", len(content))
		_, _ = hash.Write(content)
		files[relative] = fmt.Sprintf("%x", hash.Sum(nil))
		return nil
	})
	return files, err
}

// Do not discard patches made only in vendored copies of our own modules.
func (a *app) checkSourceSelfVendor(commit string) error {
	entries, err := a.treeFiles(commit, "pkg", "vendor")
	if err != nil {
		return err
	}
	for path, digest := range entries {
		if !firstPartyVendor(path) {
			continue
		}
		original := strings.TrimPrefix(strings.TrimPrefix(path, "vendor/"+upstreamModule+"/"), "vendor/"+modulePath+"/")
		value, exists := entries[original]
		if exists && value == digest {
			continue
		}
		if !exists {
			switch filepath.Base(original) {
			case "LICENSE", "NOTICE", "COPYING":
				continue
			}
		}
		return fmt.Errorf("source has a self-module vendor-only patch at %s; reconcile it with %s before release", path, original)
	}
	return nil
}

func (a *app) checkVendor(data Metadata) error {
	entries, err := a.treeFiles(data.SourceCommit, "vendor")
	if err != nil {
		return err
	}
	expected := make(map[string]string)
	for path, digest := range entries {
		if path != "vendor/modules.txt" && !firstPartyVendor(path) {
			expected[path] = digest
		}
	}
	actual, err := externalVendor(a.Root)
	if err != nil {
		return err
	}
	changed := make([]string, 0)
	for path, digest := range expected {
		if actual[path] != digest {
			changed = append(changed, path)
		}
	}
	for path := range actual {
		if _, exists := expected[path]; !exists {
			changed = append(changed, path)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	sort.Strings(changed)
	if len(changed) > 30 {
		changed = changed[:30]
	}
	return fmt.Errorf("regeneration changed OpenShift's third-party vendor files; preserve or explicitly reconcile these fixes:\n%s", strings.Join(changed, "\n"))
}

func (a *app) reconcileVendor(data Metadata) error {
	if a.gitOK("cat-file", "-e", data.SourceCommit+":vendor") {
		if _, err := a.execute(command{Dir: a.Root, Args: []string{"go", "mod", "vendor"}, Stream: true}); err != nil {
			return err
		}
	}
	return a.checkVendor(data)
}
