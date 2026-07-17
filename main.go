package main

import (
	"fmt"
	"os"

	"github.com/madcamp-official/26s-w3-c2-01/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
