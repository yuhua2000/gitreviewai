package main

import (
	"os"

	"github.com/yuhua2000/gitreviewai/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
