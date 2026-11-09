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
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	sourceRepository     = "https://github.com/openshift/prometheus-operator.git"
	downstreamRepository = "https://github.com/rhobs/obo-prometheus-operator.git"
	metadataPath         = "rhobs/release.json"
	modulePath           = "github.com/rhobs/obo-prometheus-operator"
	upstreamModule       = "github.com/prometheus-operator/prometheus-operator"
)

var (
	branchPattern  = regexp.MustCompile(`^release-(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	basePattern    = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	versionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-rhobs([1-9][0-9]*)$`)
	commitPattern  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	selfModules    = []string{modulePath, modulePath + "/pkg/apis/monitoring", modulePath + "/pkg/client"}
)

// Metadata preserves the original JSON schema, including the distinction between
// a planning-only changed=false field and metadata with no changed field.
type Metadata struct {
	SchemaVersion        int     `json:"schema_version"`
	SourceRepository     string  `json:"source_repository"`
	SourceBranch         string  `json:"source_branch"`
	SourceCommit         string  `json:"source_commit"`
	SourceVersion        string  `json:"source_version"`
	BaseVersion          string  `json:"base_version"`
	Version              string  `json:"version"`
	PreviousVersion      string  `json:"previous_version"`
	PreviousSourceCommit *string `json:"previous_source_commit"`
	ScriptsCommit        string  `json:"scripts_commit"`
	CICommit             string  `json:"ci_commit,omitempty"`
	Changed              *bool   `json:"changed,omitempty"`
}

type planOptions struct {
	Mode, Source, Downstream, PreviousVersion, SourceCommit, ScriptsRef string
	Force                                                               bool
}

func defaultPlanOptions() planOptions {
	return planOptions{Downstream: downstreamRepository, ScriptsRef: "HEAD"}
}

func versionKey(value string) ([4]uint64, error) {
	var key [4]uint64
	match := versionPattern.FindStringSubmatch(value)
	if match == nil {
		return key, fmt.Errorf("invalid downstream version: %s", value)
	}
	for i := range key {
		n, err := strconv.ParseUint(match[i+1], 10, 64)
		if err != nil {
			return key, fmt.Errorf("invalid downstream version %q: %w", value, err)
		}
		key[i] = n
	}
	return key, nil
}

func compareVersion(a, b [4]uint64) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func normalizeVersion(value string) (string, error) {
	if _, err := versionKey(value); err != nil {
		return "", err
	}
	return strings.TrimPrefix(value, "v"), nil
}

func releaseTags(version string) []string {
	version = strings.TrimPrefix(version, "v")
	return []string{"v" + version, "pkg/apis/monitoring/v" + version, "pkg/client/v" + version}
}

func releasedVersions(refs map[string]string) ([]string, error) {
	versions := make([]string, 0)
	keys := make(map[string][4]uint64)
	for ref := range refs {
		if !strings.HasPrefix(ref, "refs/tags/v") || !versionPattern.MatchString(strings.TrimPrefix(ref, "refs/tags/")) {
			continue
		}
		version := strings.TrimPrefix(ref, "refs/tags/v")
		key, err := versionKey(version)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
		keys[version] = key
	}
	sort.Slice(versions, func(i, j int) bool { return compareVersion(keys[versions[i]], keys[versions[j]]) < 0 })
	return versions, nil
}

func latestBranch(refs map[string]string) (string, error) {
	latest := ""
	var highest [2]uint64
	for ref := range refs {
		branch := strings.TrimPrefix(ref, "refs/heads/")
		match := branchPattern.FindStringSubmatch(branch)
		if match == nil {
			continue
		}
		var key [2]uint64
		for i := range key {
			value, err := strconv.ParseUint(match[i+1], 10, 64)
			if err != nil {
				return "", fmt.Errorf("invalid source branch %q: %w", branch, err)
			}
			key[i] = value
		}
		if latest == "" || key[0] > highest[0] || key[0] == highest[0] && key[1] > highest[1] {
			latest, highest = branch, key
		}
	}
	if latest == "" {
		return "", fmt.Errorf("no release-x.y branches found in the source repository")
	}
	return latest, nil
}

func nextVersion(base string, refs map[string]string) (string, error) {
	if !basePattern.MatchString(base) {
		return "", fmt.Errorf("expected a stable operator VERSION, got %q", base)
	}
	versions, err := releasedVersions(refs)
	if err != nil {
		return "", err
	}
	var revision uint64
	for _, version := range versions {
		if strings.Split(version, "-rhobs")[0] == base {
			key, err := versionKey(version)
			if err != nil {
				return "", err
			}
			if key[3] > revision {
				revision = key[3]
			}
		}
	}
	if revision == math.MaxUint64 {
		return "", fmt.Errorf("downstream revision overflow for %s", base)
	}
	version := fmt.Sprintf("%s-rhobs%d", base, revision+1)
	for _, tag := range releaseTags(version) {
		if _, exists := refs["refs/tags/"+tag]; exists {
			return "", fmt.Errorf("partial publication already exists for %s; recover it before allocating another release", version)
		}
	}
	return version, nil
}

func validateMetadata(data Metadata) error {
	if data.SchemaVersion != 1 || data.SourceRepository == "" || data.SourceCommit == "" || data.SourceVersion == "" || data.BaseVersion == "" || data.Version == "" || data.PreviousVersion == "" || data.ScriptsCommit == "" {
		return fmt.Errorf("missing or unsupported release metadata")
	}
	if !branchPattern.MatchString(data.SourceBranch) {
		return fmt.Errorf("invalid source branch in release metadata")
	}
	if !commitPattern.MatchString(data.SourceCommit) || !commitPattern.MatchString(data.ScriptsCommit) {
		return fmt.Errorf("invalid source_commit or scripts_commit in release metadata")
	}
	if data.CICommit != "" && !commitPattern.MatchString(data.CICommit) {
		return fmt.Errorf("invalid ci_commit in release metadata")
	}
	if !basePattern.MatchString(data.BaseVersion) || data.SourceVersion != data.BaseVersion {
		return fmt.Errorf("source VERSION differs from the release series base version")
	}
	version, err := normalizeVersion(data.Version)
	if err != nil {
		return err
	}
	if strings.Split(version, "-rhobs")[0] != data.BaseVersion {
		return fmt.Errorf("downstream version does not match the series base version")
	}
	if strings.HasPrefix(data.Version, "v") || strings.HasPrefix(data.PreviousVersion, "v") {
		return fmt.Errorf("metadata versions must not include the tag's v prefix")
	}
	_, err = normalizeVersion(data.PreviousVersion)
	return err
}

func loadMetadata(path string) (Metadata, error) {
	var data Metadata
	contents, err := os.ReadFile(path)
	if err != nil {
		return data, err
	}
	if err := json.Unmarshal(contents, &data); err != nil {
		return data, fmt.Errorf("invalid release metadata in %s: %w", path, err)
	}
	return data, validateMetadata(data)
}

func (a *app) readMetadata(commit string) (*Metadata, error) {
	contents, err := a.gitShow(commit, metadataPath)
	if err != nil {
		return nil, nil // Old downstream releases predate source metadata.
	}
	var data Metadata
	if err := json.Unmarshal([]byte(contents), &data); err != nil {
		return nil, fmt.Errorf("invalid release metadata at %s: %w", commit, err)
	}
	if err := validateMetadata(data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (a *app) planRelease(options planOptions) (Metadata, error) {
	var data Metadata
	refs, err := a.remoteRefs(options.Downstream, "tags")
	if err != nil {
		return data, err
	}
	versions, err := releasedVersions(refs)
	if err != nil {
		return data, err
	}
	if len(versions) == 0 {
		return data, fmt.Errorf("a previously published downstream release is required as the upgrade baseline")
	}
	previous := versions[len(versions)-1]
	if options.PreviousVersion != "" {
		previous, err = normalizeVersion(options.PreviousVersion)
		if err != nil {
			return data, err
		}
	}
	if _, exists := refs["refs/tags/v"+previous]; !exists {
		return data, fmt.Errorf("previous release v%s does not exist in %s", previous, options.Downstream)
	}
	previousCommit, err := a.fetchRef(options.Downstream, "refs/tags/v"+previous, "refs/rhobs/releases/"+previous)
	if err != nil {
		return data, err
	}
	previousData, err := a.readMetadata(previousCommit)
	if err != nil {
		return data, err
	}
	if previousData != nil {
		if previousData.Version != previous {
			return data, fmt.Errorf("previous release tag and metadata disagree")
		}
		for _, tag := range releaseTags(previous) {
			if _, exists := refs["refs/tags/"+tag]; !exists {
				return data, fmt.Errorf("previous release has partial tag publication; recover it before preparing another release")
			}
		}
	}
	source := options.Source
	var branch string
	if options.Mode == "rebuild" {
		if previousData == nil {
			return data, fmt.Errorf("legacy releases have no source metadata; start a new series with --mode new")
		}
		if source != "" && source != previousData.SourceRepository {
			return data, fmt.Errorf("a rebuild cannot change its source repository")
		}
		source, branch = previousData.SourceRepository, previousData.SourceBranch
		latest := ""
		for _, version := range versions {
			if strings.Split(version, "-rhobs")[0] == previousData.BaseVersion {
				latest = version
			}
		}
		if previous != latest {
			return data, fmt.Errorf("rebuild from the latest published revision, v%s, not v%s", latest, previous)
		}
	} else {
		if source == "" {
			source = sourceRepository
		}
		heads, err := a.remoteRefs(source, "heads")
		if err != nil {
			return data, err
		}
		branch, err = latestBranch(heads)
		if err != nil {
			return data, err
		}
	}
	tip, err := a.fetchRef(source, "refs/heads/"+branch, "refs/rhobs/source/"+branch)
	if err != nil {
		return data, err
	}
	commit := tip
	if options.SourceCommit != "" {
		if !commitPattern.MatchString(options.SourceCommit) {
			return data, fmt.Errorf("--source-commit must be a full lowercase commit SHA")
		}
		commit, err = a.git("rev-parse", options.SourceCommit+"^{commit}")
		if err != nil {
			return data, err
		}
		commit = strings.TrimSpace(commit)
		if !a.gitOK("merge-base", "--is-ancestor", commit, tip) {
			return data, fmt.Errorf("requested source commit is not on the selected release branch")
		}
	}
	base, err := a.gitShow(commit, "VERSION")
	if err != nil {
		return data, err
	}
	base = strings.TrimSpace(base)
	if !basePattern.MatchString(base) {
		return data, fmt.Errorf("source VERSION is not a stable operator version: %q", base)
	}
	if options.Mode == "rebuild" {
		if base != previousData.BaseVersion {
			return data, fmt.Errorf("source VERSION changed; explicitly start a new release series instead of rebuilding")
		}
		if !a.gitOK("merge-base", "--is-ancestor", previousData.SourceCommit, commit) {
			return data, fmt.Errorf("source history was rewritten or the requested snapshot is older than the last release")
		}
	}
	scripts, err := a.git("rev-parse", options.ScriptsRef+"^{commit}")
	if err != nil {
		return data, err
	}
	scripts = strings.TrimSpace(scripts)
	if _, err := a.gitShow(scripts, "rhobs/make-release-commit.sh"); err != nil {
		return data, err
	}
	version, err := nextVersion(base, refs)
	if err != nil {
		return data, err
	}
	nextKey, err := versionKey(version)
	if err != nil {
		return data, err
	}
	previousKey, err := versionKey(previous)
	if err != nil {
		return data, err
	}
	if compareVersion(nextKey, previousKey) <= 0 {
		return data, fmt.Errorf("the selected source would downgrade the previous downstream release")
	}
	changed := options.Mode == "new" || options.Force
	var previousSource *string
	if previousData != nil {
		previousSource = &previousData.SourceCommit
		changed = changed || commit != previousData.SourceCommit
	}
	return Metadata{
		SchemaVersion: 1, SourceRepository: source, SourceBranch: branch, SourceCommit: commit,
		SourceVersion: base, BaseVersion: base, Version: version, PreviousVersion: previous,
		PreviousSourceCommit: previousSource, ScriptsCommit: scripts, Changed: &changed,
	}, nil
}

func (a *app) verifyRelease() (Metadata, error) {
	data, err := loadMetadata(filepath.Join(a.Root, metadataPath))
	if err != nil {
		return data, err
	}
	version, err := readText(filepath.Join(a.Root, "VERSION"))
	if err != nil {
		return data, err
	}
	if strings.TrimSpace(version) != data.Version {
		return data, fmt.Errorf("VERSION and release metadata disagree")
	}
	sourceVersion, err := a.gitShow(data.SourceCommit, "VERSION")
	if err != nil {
		return data, err
	}
	if strings.TrimSpace(sourceVersion) != data.SourceVersion {
		return data, fmt.Errorf("pinned source VERSION and release metadata disagree")
	}
	if !a.gitOK("merge-base", "--is-ancestor", data.SourceCommit, "HEAD") {
		return data, fmt.Errorf("pinned source commit is not an ancestor of the release")
	}
	branch, err := a.git("branch", "--show-current")
	if err != nil {
		return data, err
	}
	branch = strings.TrimSpace(branch)
	if strings.HasPrefix(branch, "rhobs-rel-") && branch != "rhobs-rel-"+data.Version {
		return data, fmt.Errorf("release branch and VERSION disagree")
	}
	if ref := os.Getenv("GITHUB_REF"); strings.HasPrefix(ref, "refs/heads/rhobs-rel-") && ref != "refs/heads/rhobs-rel-"+data.Version {
		return data, fmt.Errorf("GitHub release branch and VERSION disagree")
	}
	return data, a.checkVendor(data)
}
