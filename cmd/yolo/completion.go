package main

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kido5217/yolo/internal/config"
)

// Dynamic shell completion (yolo-k49, the v0.6.0 D1 "static only" follow-up):
// candidate sources for --profile (config.List, no DB) and the root
// positional sessionID (the storage DB of the resolved --dir). A completion
// request runs in a short-lived process spawned by the shell, so the
// functions must be read-only (no profile creation, no EnsureActive), quiet
// (any error yields no candidates, never a stderr message), and cheap.

// sessionIDCandidateLimit caps the sessionID candidates (most-recently
// updated first — the recent sessions are the useful ones).
const sessionIDCandidateLimit = 50

// profileCandidates returns every profile id and (when different) name under
// the global yolo root — the same source --profile resolves against
// (ProcessProfile). Read-only on purpose: a completion request must not
// create the default profile (a fresh root yields no candidates).
func profileCandidates() []string {
	root, err := config.GlobalYoloDir()
	if err != nil {
		return nil
	}
	profiles, err := config.List(root)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(profiles)*2)
	for _, p := range profiles {
		out = append(out, p.ID)
		if p.Name != p.ID {
			out = append(out, p.Name)
		}
	}
	return out
}

// profileCompletionFunc is the --profile flag completion (root TUI command
// and serve).
func profileCompletionFunc(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return profileCandidates(), cobra.ShellCompDirectiveNoFileComp
}

// sessionIDCandidates returns the session ids of workDir from the storage DB
// under the XDG data dir — the same single store buildDeps opens (openDB on
// <data>/yolo/storage/yolo.db), most-recently updated first, capped.
func sessionIDCandidates(workDir string) []string {
	dataDir, err := config.DataYoloDir()
	if err != nil {
		return nil
	}
	db, err := openDB(filepath.Join(dataDir, "storage", "yolo.db"))
	if err != nil {
		return nil
	}
	defer db.Close()
	rows, err := db.ListSessions(context.Background(), workDir, sessionIDCandidateLimit)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

// sessionIDCompletionFunc is the root positional (yolo [sessionID])
// completion: the sessions of the --dir store (default CWD).
func sessionIDCompletionFunc(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	dir, _ := cmd.Flags().GetString("dir")
	wd, err := workDir(dir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return sessionIDCandidates(wd), cobra.ShellCompDirectiveNoFileComp
}
