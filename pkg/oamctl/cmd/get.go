package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(getCmd)
}

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Display one or many resources",
	Long:  `Display one or many resources`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("get get get... ;o) ")
	},
}
