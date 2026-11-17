package main

import (
	"fmt"
	"os"

	"github.com/fabianoflorentino/seraph/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "seraph:", err)
		os.Exit(1)
	}
}
