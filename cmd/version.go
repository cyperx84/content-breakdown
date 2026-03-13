package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print breakdown version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("breakdown v0.1.0-mvp")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
