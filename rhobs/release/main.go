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

// The release command plans and prepares RHOBS releases from pinned OpenShift
// branch snapshots. It uses only the standard library and Git/Go executables.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if err := runCLI(os.Args[1:], os.Stdout, os.Stderr); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "Release error:", err)
		os.Exit(1)
	}
}

func runCLI(args []string, out, stderr io.Writer) error {
	global := flag.NewFlagSet("rhobs-release", flag.ContinueOnError)
	global.SetOutput(stderr)
	root := global.String("root", "", "Repository directory (defaults to the current Git checkout)")
	global.Usage = func() {
		fmt.Fprintln(stderr, "Usage: rhobs/release.sh COMMAND [OPTIONS]")
		fmt.Fprintln(stderr, "Commands: plan, prepare, transform, verify, check-vendor, reconcile-vendor, assert-clean, publish-tags, check-import, go-version, sync-ci")
		global.PrintDefaults()
	}
	if err := global.Parse(args); err != nil {
		return err
	}
	if global.NArg() == 0 {
		global.Usage()
		return flag.ErrHelp
	}
	a := newApp(*root, out, stderr)
	path, err := a.git("rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	a.Root = strings.TrimSpace(path)
	return a.dispatch(global.Args())
}

func parseFlags(flags *flag.FlagSet, args []string) error {
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	return nil
}

func addPlanFlags(flags *flag.FlagSet, options *planOptions) {
	flags.StringVar(&options.Mode, "mode", "", "new (highest source branch) or rebuild (recorded branch)")
	flags.StringVar(&options.Source, "source", "", "Source repository URL; rebuilds use recorded provenance")
	flags.StringVar(&options.Downstream, "downstream", downstreamRepository, "Downstream repository URL")
	flags.StringVar(&options.PreviousVersion, "previous-version", "", "Upgrade baseline; defaults to the latest numeric downstream root tag")
	flags.StringVar(&options.SourceCommit, "source-commit", "", "Exact full source commit SHA on the selected branch")
	flags.StringVar(&options.ScriptsRef, "scripts-ref", "HEAD", "Committed RHOBS overlay ref")
	flags.BoolVar(&options.Force, "force", false, "Rebuild even when the source SHA is unchanged")
}

func (a *app) dispatch(args []string) error {
	name := args[0]
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(a.Err)
	switch name {
	case "plan", "prepare":
		options := prepareOptions{planOptions: defaultPlanOptions()}
		addPlanFlags(flags, &options.planOptions)
		if name == "prepare" {
			flags.StringVar(&options.PlanFile, "plan", "", "Reuse a pinned JSON plan without rediscovering source tips")
			flags.StringVar(&options.Worktree, "worktree", "", "New worktree directory; defaults to a temporary directory")
			flags.StringVar(&options.ForkRepository, "fork-repository", "", "GitHub owner/repo for the PR branch")
			flags.BoolVar(&options.Execute, "execute", false, "Prepare a local worktree; otherwise only print the plan")
			flags.BoolVar(&options.NoChecks, "no-checks", false, "Skip optional local lint/unit checks, never required CI")
			flags.BoolVar(&options.PublishPR, "publish-pr", false, "Push branches and open a PR (also requires --execute)")
		}
		if err := parseFlags(flags, args[1:]); err != nil {
			return err
		}
		if options.Mode != "" && options.Mode != "new" && options.Mode != "rebuild" {
			return fmt.Errorf("--mode must be new or rebuild")
		}
		if name == "plan" {
			if options.Mode == "" {
				return fmt.Errorf("--mode is required")
			}
			data, err := a.planRelease(options.planOptions)
			if err != nil {
				return err
			}
			return writeJSON(a.Out, data)
		}
		data, err := a.preparePlan(options)
		if err != nil {
			return err
		}
		return a.prepareRelease(options, data)
	case "transform", "go-version":
		path := flags.String("plan", "", "Pinned JSON release plan")
		if err := parseFlags(flags, args[1:]); err != nil {
			return err
		}
		if *path == "" {
			return fmt.Errorf("--plan is required")
		}
		data, err := loadMetadata(a.repoPath(*path))
		if err != nil {
			return err
		}
		if name == "transform" {
			return a.transform(data)
		}
		version, err := a.sourceGoVersion(data)
		if err == nil {
			_, err = fmt.Fprintln(a.Out, version)
		}
		return err
	case "verify", "check-vendor", "reconcile-vendor", "assert-clean":
		if err := parseFlags(flags, args[1:]); err != nil {
			return err
		}
		if name == "assert-clean" {
			return a.assertClean()
		}
		if name == "verify" {
			data, err := a.verifyRelease()
			if err != nil {
				return err
			}
			return writeJSON(a.Out, data)
		}
		data, err := loadMetadata(filepath.Join(a.Root, metadataPath))
		if err != nil {
			return err
		}
		if name == "reconcile-vendor" {
			return a.reconcileVendor(data)
		}
		return a.checkVendor(data)
	case "publish-tags":
		remote := flags.String("remote", "origin", "Downstream publication remote")
		if err := parseFlags(flags, args[1:]); err != nil {
			return err
		}
		return a.publishTags(*remote)
	case "check-import":
		published := flags.Bool("published", false, "Import published modules without local replacements")
		if err := parseFlags(flags, args[1:]); err != nil {
			return err
		}
		return a.checkImport(*published)
	case "sync-ci":
		ref := flags.String("ref", "", "Pinned source commit/ref whose upstream CI should be restored")
		if err := parseFlags(flags, args[1:]); err != nil {
			return err
		}
		if *ref == "" {
			return fmt.Errorf("--ref is required")
		}
		commit, err := a.git("rev-parse", "--verify", *ref+"^{commit}")
		if err != nil {
			return err
		}
		ci, err := a.syncCI(strings.TrimSpace(commit), a.Root)
		if err == nil {
			_, err = fmt.Fprintln(a.Out, "Restored upstream CI from", ci)
		}
		return err
	default:
		return fmt.Errorf("unknown command %q", name)
	}
}
