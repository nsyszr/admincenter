package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/nsyszr/admincenter/pkg/api"
	"github.com/spf13/cobra"
)

var flagCLIFile string

func init() {
	cliCmd.Flags().StringVarP(&flagCLIFile, "file", "f", "", "Load file with commands to run")
	rootCmd.AddCommand(cliCmd)
}

var cliCmd = &cobra.Command{
	Use:   "cli [realm] ([command] | -f file)",
	Short: "Run a cli command",
	Long:  `Run a cli command`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// fmt.Println("run run run... ;o) ")
		if flagCLIFile == "" && len(args) < 2 {
			fmt.Println("Missing command or file")
			cmd.Help()
			os.Exit(1)
		}

		command := ""
		realms := []string{}
		if flagCLIFile != "" {
			b, _ := ioutil.ReadFile(flagCLIFile)
			command = string(b)
			realms = args

		} else {
			command = args[len(args)-1]
			realms = args[:len(args)-1]
		}
		fmt.Printf("Run command %s on %v\n", command, realms)

		runRequest := api.SessionRunRequest{
			Realms:  realms,
			Command: command,
		}
		js, _ := jsonMarshal(runRequest) // json.Marshal(runRequest)
		log.Printf("req: %s", js)

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

		for _, result := range runResponse.Realms {
			// fmt.Printf("Result for %s:\n", realm)
			s, _ := json.Marshal(result.Results)
			var output api.RPCM3CLIResult
			json.Unmarshal(s, &output)
			for _, line := range output.Output {
				fmt.Println(line)
			}
			// fmt.Printf("%s\n\n", s)
		}
	},
}

func jsonMarshal(t interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}
