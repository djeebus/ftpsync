package main

import (
	"fmt"
	"os"

	"github.com/djeebus/ftpsync/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println("error: ", err)
		os.Exit(1)
	}
}
