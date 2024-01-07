package main

import (
	"github.com/umlx5h/gtrash/internal/cmd"
)

// set by CI
var (
	version = "unknown"
	commit  = "unknown"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	cmd.Execute(cmd.Version{
		Version: version,
		Commit:  commit,
		Date:    date,
		BuiltBy: builtBy,
	})
}
