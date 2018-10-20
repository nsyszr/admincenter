package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/nsyszr/admincenter/pkg/api"
	"github.com/spf13/cobra"
)

var flagRunFile string

func init() {
	runCmd.Flags().StringVarP(&flagRunFile, "file", "f", "", "Load file with commands to run")
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run [realm, ...] ([command] | -f file)",
	Short: "Run a command",
	Long:  `Run a command`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// fmt.Println("run run run... ;o) ")
		if flagRunFile == "" && len(args) < 2 {
			fmt.Println("Missing command or file")
			cmd.Help()
			os.Exit(1)
		}

		command := args[len(args)-1]
		/*if flagRunFile != "" {
			fmt.Printf("Run file %s on %v\n", flagRunFile, args)
		} else {
			fmt.Printf("Run command '%s' on %v\n", args[len(args)-1], args[:len(args)-1])
		}*/

		runRequest := api.SessionRunRequest{
			Realms:  args[:len(args)-1],
			Command: command,
		}
		js, _ := json.Marshal(runRequest)
		//log.Println("req: ", string(js))

		response, err := http.Post("http://localhost:8080/api/v1/sessions/run",
			"application/json", bytes.NewBuffer(js))
		//response, err := http.Get("http://localhost:8080/api/v1/sessions/active")
		if err != nil {
			fmt.Printf("The HTTP request failed with error %s\n", err)
			return
		}
		/*	data, _ := ioutil.ReadAll(response.Body)
			fmt.Println(string(data))
		}*/

		/*bodyBytes, _ := ioutil.ReadAll(response.Body)
		bodyString := string(bodyBytes)
		log.Println("body: ", bodyString)*/

		runResponse := api.SessionRunResponse{}
		//log.Println("resp: ", runResponse)

		err = json.NewDecoder(response.Body).Decode(&runResponse)
		if err != nil {
			fmt.Printf("Failed to decode response %s\n", err)
			return
		}

		for realm, result := range runResponse.Realms {
			fmt.Printf("Result for %s:\n", realm)
			s, _ := json.Marshal(result)
			fmt.Printf("%s\n\n", s)
		}
	},
}
