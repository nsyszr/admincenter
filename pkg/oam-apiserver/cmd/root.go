package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "oam-apiserver",
	Short: "OAM API-Server",
}

/*func init() {
	// TODO: Add config file here
}*/

// Execute runs the oam-apiserver command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
