package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version info, injected at build time via -ldflags.
var (
	version = "dev"
	commit  = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gitreviewai %s (commit: %s)\n", version, commit)
	},
}
