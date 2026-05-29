package main

import (
	"os"

	"github.com/Life-USTC/CLI/internal/cmd/root"
	"github.com/Life-USTC/CLI/internal/output"
)

func main() {
	cmd := root.NewCmdRoot()
	if err := cmd.Execute(); err != nil {
		output.Errorf("%s", err)
		output.Hint("run 'life-ustc <command> --help' for usage information")
		os.Exit(1)
	}
}
