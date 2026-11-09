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
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type prepareOptions struct {
	planOptions
	PlanFile, Worktree, ForkRepository string
	Execute, NoChecks, PublishPR       bool
}

func githubRepository(value string) (string, error) {
	match := regexp.MustCompile(`^(?:https://github\.com/|git@github\.com:)?([\w.-]+/[\w.-]+?)(?:\.git)?$`).FindStringSubmatch(value)
	if match == nil {
		return "", fmt.Errorf("expected a GitHub repository URL or owner/repo, got %q", value)
	}
	return match[1], nil
}

func (a *app) preparePlan(options prepareOptions) (Metadata, error) {
	var data Metadata
	if options.PublishPR && !options.Execute {
		return data, fmt.Errorf("--publish-pr requires --execute")
	}
	if options.PlanFile == "" {
		if options.Mode == "" {
			return data, fmt.Errorf("specify --mode or --plan")
		}
		return a.planRelease(options.planOptions)
	}
	data, err := loadMetadata(a.repoPath(options.PlanFile))
	if err != nil {
		return data, err
	}
	if data.Changed == nil {
		return data, fmt.Errorf("release plan must include a boolean changed field")
	}
	if options.Mode != "" || options.Source != "" || options.SourceCommit != "" || options.PreviousVersion != "" || options.Force || options.ScriptsRef != "HEAD" {
		return data, fmt.Errorf("--plan cannot be combined with source/version selection arguments")
	}
	if *data.Changed {
		refs, err := a.remoteRefs(options.Downstream, "tags")
		if err != nil {
			return data, err
		}
		next, err := nextVersion(data.BaseVersion, refs)
		if err != nil {
			return data, err
		}
		if next != data.Version {
			return data, fmt.Errorf("release plan is stale; another revision has been published")
		}
	}
	return data, nil
}

func (a *app) prepareRelease(options prepareOptions, data Metadata) error {
	if options.PublishPR && !options.Execute {
		return fmt.Errorf("--publish-pr requires --execute")
	}
	if err := writeJSON(a.Out, data); err != nil {
		return err
	}
	if data.Changed == nil {
		return fmt.Errorf("release plan must include a boolean changed field")
	}
	if !*data.Changed {
		fmt.Fprintln(a.Err, "Source is unchanged; no rebuild needed")
		return nil
	}
	if !options.Execute {
		fmt.Fprintln(a.Err, "Dry run: use --execute to prepare a local release checkout")
		return nil
	}
	if err := a.assertClean(); err != nil {
		return err
	}
	target, branch := "rhobs-rel-"+data.Version, "rhobs-pr-"+data.Version
	var repository, fork string
	if options.PublishPR {
		if options.ForkRepository == "" {
			return fmt.Errorf("--publish-pr requires --fork-repository owner/repo")
		}
		var err error
		repository, err = githubRepository(options.Downstream)
		if err != nil {
			return err
		}
		fork, err = githubRepository(options.ForkRepository)
		if err != nil {
			return err
		}
		output, err := a.run(a.Root, "gh", "api", "--method", "GET", "repos/"+repository+"/pulls",
			"-f", "state=open", "-f", "base="+target, "-f", "head="+strings.Split(fork, "/")[0]+":"+branch)
		if err != nil {
			return err
		}
		var existing []struct {
			URL string `json:"html_url"`
		}
		if err := json.Unmarshal([]byte(output), &existing); err != nil {
			return err
		}
		if len(existing) != 0 {
			fmt.Fprintln(a.Out, existing[0].URL)
			return nil
		}
	}
	worktree := options.Worktree
	var err error
	if worktree == "" {
		worktree, err = os.MkdirTemp(temporaryParent(), "rhobs-release-")
	} else {
		worktree, err = filepath.Abs(a.repoPath(worktree))
	}
	if err != nil {
		return err
	}
	if _, err := a.git("worktree", "add", "-b", branch, worktree, data.SourceCommit); err != nil {
		return err
	}
	fmt.Fprintln(a.Err, "Release checkout:", worktree)
	child := newApp(worktree, a.Out, a.Err)
	child.Runner = a.Runner
	if err := child.applyOverlay(data.ScriptsCommit); err != nil {
		return err
	}
	if _, err := child.git("commit", "-s", "-m", "git: apply rhobs-scripts overlay"); err != nil {
		return err
	}
	plan, err := os.CreateTemp(temporaryParent(), "rhobs-plan-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(plan.Name())
	writeErr := writeJSON(plan, data)
	closeErr := plan.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	args := []string{"bash", "rhobs/make-release-commit.sh", "--plan", plan.Name()}
	if options.NoChecks {
		args = append(args, "--no-checks")
	}
	if _, err := a.execute(command{Dir: worktree, Args: args, Stream: true}); err != nil {
		return err
	}
	if !options.PublishPR {
		fmt.Fprintf(a.Err, "Prepared %s; nothing has been pushed\n", branch)
		return nil
	}
	heads, err := a.remoteRefs(options.Downstream, "heads")
	if err != nil {
		return err
	}
	if existing, exists := heads["refs/heads/"+target]; exists && existing != data.SourceCommit {
		return fmt.Errorf("%s already exists at a different commit; refusing to replace it", target)
	}
	// Never force-push the release base or a contributor's PR branch.
	if _, err := child.git("push", options.Downstream, data.SourceCommit+":refs/heads/"+target); err != nil {
		return err
	}
	if _, err := child.git("push", "https://github.com/"+fork+".git", "HEAD:refs/heads/"+branch); err != nil {
		return err
	}
	body := fmt.Sprintf("Source: %s (%s)\n\nSource commit: `%s`\n\nScripts commit: `%s`\n\nUpgrade baseline: `v%s`\n",
		data.SourceRepository, data.SourceBranch, data.SourceCommit, data.ScriptsCommit, data.PreviousVersion)
	if data.PreviousSourceCommit != nil {
		body += fmt.Sprintf("\nSource changes: %s/compare/%s...%s\n", strings.TrimSuffix(data.SourceRepository, ".git"), *data.PreviousSourceCommit, data.SourceCommit)
	}
	output, err := a.run(a.Root, "gh", "pr", "create", "--repo", repository, "--base", target,
		"--head", strings.Split(fork, "/")[0]+":"+branch, "--title", "chore(release): v"+data.Version, "--body", body)
	if err == nil {
		fmt.Fprintln(a.Out, strings.TrimSpace(output))
	}
	return err
}
