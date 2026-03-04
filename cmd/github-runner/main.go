// Package main is the entry point for the self-hosted GitHub Actions runner.
// It delegates all CLI handling to the internal/cli package and exits with
// the appropriate exit code.
package main

import (
	"os"

	"github.com/org/github-runner/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
