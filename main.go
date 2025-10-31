package main

import (
	"fmt"
	"os"

	"github.com/djeebus/ftpsync/cmd"
)

func main() {
	if err := cmd.RootCmd(); err != nil {
		fmt.Println("error: ", err)
		os.Exit(1)
	}
}
