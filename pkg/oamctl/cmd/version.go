package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the client and server information for the current context",
	Long:  `Print the client and server information for the current context`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Client Version: 0.1.0")
		fmt.Println("Server Version: not implemented yet")
	},
}
