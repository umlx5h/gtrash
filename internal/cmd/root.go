package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"
	"github.com/umlx5h/gtrash/internal/env"
	"golang.org/x/term"
)

var (
	progName    = filepath.Base(os.Args[0])
	errContinue = errors.New("")

	isTerminal bool
)

func init() {
	if term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd())) {
		isTerminal = true
	}
}

func Execute(version Version) {
	err := newRootCmd(version).cmd.Execute()
	if err != nil {
		if !errors.Is(err, errContinue) {
			fmt.Fprintf(os.Stderr, "%s: error: %s\n", progName, err)
		}
		os.Exit(1)
	}
}

type Version struct {
	Version string
	Commit  string
	Date    string
	BuiltBy string
}

func (v Version) Print() string {
	var s strings.Builder
	fmt.Fprintln(&s, "gtrash: Trash CLI Manager written in Go")
	fmt.Fprintln(&s, "https://github.com/umlx5h/gtrash")
	fmt.Fprintln(&s, "")
	fmt.Fprintln(&s, "version: "+v.Version)
	fmt.Fprintln(&s, "commit: "+v.Commit)
	fmt.Fprintln(&s, "buildDate: "+v.Date)
	fmt.Fprintln(&s, "builtBy: "+v.BuiltBy)

	return s.String()
}

// global options
var (
	debug bool
)

type rootCmd struct {
	cmd *cobra.Command
}

func newRootCmd(version Version) *rootCmd {
	root := &rootCmd{}
	cmd := &cobra.Command{
		Use:           progName,
		SilenceErrors: true,
		Short:         "Trash CLI manager written in Go",
		Long: `Trash CLI manager written in Go
  https://github.com/umlx5h/gtrash`,
		Version: version.Print(),
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			// setup debug log level
			lvl := &slog.LevelVar{}

			lvl.Set(slog.LevelWarn)
			if debug {
				lvl.Set(slog.LevelDebug)
			}
			// colored format
			logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
				Level:      lvl,
				TimeFormat: "15:04:05.000",
				NoColor:    !isTerminal,
			}))

			slog.SetDefault(logger)

			slog.Debug("gtrash version", "version", fmt.Sprintf("%+v", version))
			slog.Debug("enviornment variable",
				"HOME_TRASH_DIR", env.HOME_TRASH_DIR,
				"ONLY_HOME_TRASH", env.ONLY_HOME_TRASH,
			)
		},
	}

	cmd.SetVersionTemplate("{{.Version}}")
	cmd.PersistentFlags().BoolVar(&debug, "debug", false, "debug mode")
	cmd.PersistentFlags()

	// disable help subcommand
	cmd.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})

	// prefix program name
	cmd.SetErrPrefix(fmt.Sprintf("%s: error:", progName))

	// Add subcommands
	cmd.AddCommand(
		newPutCmd().cmd,
		newFindCmd().cmd,
		newRestoreCmd().cmd,
		newRestoreGroupCmd().cmd,
		newRemoveCmd().cmd,
		newSummaryCmd().cmd,
		newMetafixCmd().cmd,
	)
	root.cmd = cmd
	return root
}
