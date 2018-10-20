package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "oamctl",
	Short: "Control and manage OAM thru CLI",
	Long: `Currently I have no ideas for a long description. Michl would say, don't waste 
time into lot's of bla bla.... and he would say, C is the best programming 
language ever. Well... it depends. *laugh* I prefer Golang for such tools like 
oamctl more. ;) Just my 2 fucking cents. In the end I spent currently my free 
time into developing a tool for Michl. ;) So I don't care about if Michl likes
C more than me. :o)))) Oh fuck... and perhaps Michl will read this nice long 
description for oamctl. ;) Servus Michl. :o))))`,

	Run: func(cmd *cobra.Command, args []string) {
		log.Print("oamctl called...")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
