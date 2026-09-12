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

func TestReviewsWaitTimeoutCommand(t *testing.T) {
	if os.Getenv("GH_HELPER_TEST_TIMEOUT_COMMAND") == "1" {
		// Keep real argument/flag dispatch; replace only the network operation
		// with the same timeout calculation used by the waiter.
		waitReviewsCmd.RunE = func(cmd *cobra.Command, args []string) error {
			duration, _, err := calculateEffectiveTimeout()
			if err == nil {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), duration)
			}
			return err
		}
		rootCmd.SetArgs(os.Args[slices.Index(os.Args, "--")+1:])
		if err := rootCmd.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	for _, tt := range []struct {
		name      string
		args      []string
		want      string
		wantError bool
	}{
		{"default", []string{"reviews", "wait", "123"}, "5m0s", false},
		{"before commands", []string{"--timeout", "45s", "reviews", "wait", "123"}, "45s", false},
		{"after commands", []string{"reviews", "wait", "123", "--timeout", "45s"}, "45s", false},
		{"fractional", []string{"reviews", "wait", "123", "--timeout=1.5m"}, "1m30s", false},
		{"invalid before", []string{"--timeout=invalid", "reviews", "wait", "123"}, "invalid timeout format", true},
		{"invalid after", []string{"reviews", "wait", "123", "--timeout=invalid"}, "invalid timeout format", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"-test.run=^TestReviewsWaitTimeoutCommand$", "--"}, tt.args...)
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GH_HELPER_TEST_TIMEOUT_COMMAND=1", "BASH_MAX_TIMEOUT_MS=", "BASH_DEFAULT_TIMEOUT_MS=")
			out, err := cmd.CombinedOutput()
			if (err != nil) != tt.wantError {
				t.Fatalf("error = %v, wantError = %v; output: %s", err, tt.wantError, out)
			}
			if !strings.Contains(string(out), tt.want+"\n") && !tt.wantError {
				t.Fatalf("output = %q, want duration %q", out, tt.want)
			}
			if tt.wantError && !strings.Contains(string(out), tt.want) {
				t.Fatalf("output = %q, want error %q", out, tt.want)
			}
		})
	}
}
