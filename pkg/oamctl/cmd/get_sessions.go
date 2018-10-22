package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/nsyszr/admincenter/pkg/api"
	"github.com/nsyszr/admincenter/pkg/oamctl/util"
	"github.com/spf13/cobra"
)

var flagGetDevicesAll bool

func init() {
	// getSessionsCmd.Flags().BoolVarP(&flagGetDevicesAll, "all", "a", false, "Show all sessions (default shows just connected)")
	getCmd.AddCommand(getSessionsCmd)
}

var getSessionsCmd = &cobra.Command{
	Use:     "sessions",
	Aliases: []string{"sess"},
	Short:   "List sessions",
	Long:    `List sessions`,
	Run: func(cmd *cobra.Command, args []string) {
		w := util.GetNewTabWriter(os.Stdout)

		response, err := http.Get("http://localhost:8080/api/v1/sessions/active")
		if err != nil {
			fmt.Printf("The HTTP request failed with error %s\n", err)
			return
		}

		activeSessions := &api.ActiveSessions{}
		err = json.NewDecoder(response.Body).Decode(&activeSessions)
		if err != nil {
			fmt.Printf("Failed to decode response %s\n", err)
			return
		}

		// Output results to tab writer
		fmt.Fprintln(w, "REALM\tCONNECTED-SINCE\tLAST-MESSAGE\tTIMEOUT")
		for _, sess := range activeSessions.Active {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n",
				sess.Realm,
				sess.ConnectedSince.Format("2006-01-02 15:04:05"),
				sess.LastMessage.Format("2006-01-02 15:04:05"),
				sess.SessionTimeout)
		}
		fmt.Fprintln(w)
		w.Flush()
	},
}
