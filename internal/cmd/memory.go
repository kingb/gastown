package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// runBd executes a bd command with args, passing through stdout/stderr.
func runBd(args ...string) error {
	cmd := exec.Command("bd", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

var memoryKeyFlag string

var rememberCmd = &cobra.Command{
	Use:   `remember "<insight>"`,
	Short: "Store a persistent memory (passthrough to bd remember)",
	Long: `Store a memory that persists across sessions and account rotations.
Memories are injected at prime time so agents have them in every session.

This is a passthrough to 'bd remember'. Memories are stored in the beads
k/v store for the current project.

Examples:
  gt remember "always run tests with -race flag"
  gt remember --key dolt-phantoms "Dolt phantom DBs hide in three places"`,
	Args:                  cobra.ExactArgs(1),
	DisableFlagParsing:    false,
	DisableFlagsInUseLine: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		bdArgs := []string{"remember"}
		if memoryKeyFlag != "" {
			bdArgs = append(bdArgs, "--key", memoryKeyFlag)
		}
		bdArgs = append(bdArgs, args[0])
		return runBd(bdArgs...)
	},
}

var memoriesCmd = &cobra.Command{
	Use:   "memories [search]",
	Short: "List or search persistent memories (passthrough to bd memories)",
	Long: `List all memories, or search by keyword.

Examples:
  gt memories              # list all
  gt memories dolt         # search for memories about dolt`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bdArgs := []string{"memories"}
		bdArgs = append(bdArgs, args...)
		return runBd(bdArgs...)
	},
}

var forgetCmd = &cobra.Command{
	Use:   "forget <key>",
	Short: "Remove a persistent memory (passthrough to bd forget)",
	Long: `Remove a memory by its key. Use 'gt memories' to see available keys.

Examples:
  gt forget dolt-phantoms`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBd("forget", args[0])
	},
}

var recallCmd = &cobra.Command{
	Use:   "recall <key>",
	Short: "Retrieve a specific memory (passthrough to bd recall)",
	Long: `Retrieve the full content of a memory by its key.

Examples:
  gt recall dolt-phantoms`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBd("recall", args[0])
	},
}

func init() {
	rememberCmd.Flags().StringVar(&memoryKeyFlag, "key", "", "Explicit key for the memory (auto-generated from content if not set)")

	rootCmd.AddCommand(rememberCmd)
	rootCmd.AddCommand(memoriesCmd)
	rootCmd.AddCommand(forgetCmd)
	rootCmd.AddCommand(recallCmd)
}
