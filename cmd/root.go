package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "breakdown",
	Short: "Content Breakdown Workflow — source to structured findings",
	Long: `breakdown transforms source material (YouTube videos, articles, etc.)
into structured findings, lens-based synthesis, and actionable vault notes.

Quick start:
  breakdown run "https://youtube.com/watch?v=..."`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
