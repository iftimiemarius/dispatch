// Command dispatch is the entrypoint for the Dispatch work orchestration CLI.
package main

import (
	"os"

	"github.com/iftimiemarius/dispatch/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
