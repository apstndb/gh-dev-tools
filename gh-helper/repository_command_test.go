// Copyright 2026 apstndb

package main

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRepositoryFlagsCommand(t *testing.T) {
	if os.Getenv("GH_HELPER_TEST_REPOSITORY_COMMAND") == "1" {
		// Preserve real Cobra dispatch and client construction, replacing only
		// network operations. Subprocesses isolate the package-level flag state.
		for _, command := range []*cobra.Command{waitReviewsCmd, nodeIDIssueCmd, fetchReviewsCmd} {
			command.RunE = func(cmd *cobra.Command, args []string) error {
				client := NewGitHubClient(owner, repo)
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s/%s\n", client.Owner, client.Repo)
				return err
			}
		}
		rootCmd.SetArgs(os.Args[slices.Index(os.Args, "--")+1:])
		if err := rootCmd.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	for _, command := range [][]string{
		{"reviews", "wait", "123"},
		{"node-id", "issue", "123"},
		{"reviews", "fetch", "123"}, // Existing inherited-flag control.
	} {
		for _, flags := range []struct {
			name          string
			before, after []string
			want          string
		}{
			{name: "default", want: DefaultOwner + "/" + DefaultRepo},
			{name: "before", before: []string{"--owner", "fixture-owner", "--repo", "fixture-repo"}, want: "fixture-owner/fixture-repo"},
			{name: "after", after: []string{"--owner=fixture-owner", "--repo=fixture-repo"}, want: "fixture-owner/fixture-repo"},
			{name: "owner only", after: []string{"--owner=fixture-owner"}, want: "fixture-owner/" + DefaultRepo},
			{name: "repo only", after: []string{"--repo=fixture-repo"}, want: DefaultOwner + "/fixture-repo"},
		} {
			t.Run(strings.Join(command[:2], " ")+"/"+flags.name, func(t *testing.T) {
				args := []string{"-test.run=^TestRepositoryFlagsCommand$", "--"}
				args = append(args, flags.before...)
				args = append(args, command...)
				args = append(args, flags.after...)
				cmd := exec.Command(os.Args[0], args...)
				cmd.Env = append(os.Environ(), "GH_HELPER_TEST_REPOSITORY_COMMAND=1")
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("command failed: %v; output: %s", err, out)
				}
				if !strings.HasPrefix(string(out), flags.want+"\n") {
					t.Fatalf("client repository = %q, want %q", out, flags.want)
				}
			})
		}
	}
}
