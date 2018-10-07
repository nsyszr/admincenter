package main

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/nsyszr/admincenter/pkg/m3"
	yaml "gopkg.in/yaml.v2"
)

func main() {
	fmt.Println("M3 YAML to ASCII")
	fmt.Println("----------------")

	var m3Cfg m3.Config

	b, err := ioutil.ReadFile("./docs/ascii_1.yml")
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	err = yaml.Unmarshal(b, &m3Cfg)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// fmt.Printf("--- m3Cfg:\n%v\n\n", len(m3Cfg.Administration.Users.User))

	s := m3Cfg.Marshal()
	fmt.Println(s)
}
