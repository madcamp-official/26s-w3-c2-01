package main

import (
	"fmt"
	"os"

	"github.com/madcamp-official/26s-w3-c2-01/cmd"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if code := cmd.ExitCode(err); code != cmd.ExitSuccess {
		os.Exit(code)
	}
}
