package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gitreviewai",
	Short: "AI-powered GitLab Merge Request code review bot",
	Long:  `GitReviewAI is an AI-powered GitLab Merge Request code review bot that analyzes code changes and posts review comments.`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(versionCmd)
}
