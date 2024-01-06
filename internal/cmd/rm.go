package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/umlx5h/gtrash/internal/glog"
	"github.com/umlx5h/gtrash/internal/trash"
	"github.com/umlx5h/gtrash/internal/tui"
)

type removeCmd struct {
	cmd  *cobra.Command
	opts removeOptions
}

type removeOptions struct{}

func newRemoveCmd() *removeCmd {
	root := &removeCmd{}
	cmd := &cobra.Command{
		Use:   "rm PATH...",
		Short: "Remove trashed files PERMANENTLY from cmd args",
		Long: `Descricption:
  Remove the file passed as a command line argument.
  The path must be specified as a full path.

  This command is intended to be used in combination with other commands such as fzf.
  It is usually better to use find --rm.`,
		Example: `  # Remove files matching the full path PERMANENTLY.
  $ gtrash rm /home/user/file1 /home/user/file2

  # fuzzy find multiple items and remove them PERMANENTLY.
  # The -o in xargs is required to display the confirmation prompt.
  $ gtrash find | fzf --multi | awk -F'\t' '{print $2}' | xargs -o gtrash rm`,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := removeCmdRun(args, root.opts); err != nil {
				return err
			}
			if glog.ExitCode() > 0 {
				return errContinue
			}
			return nil
		},
	}

	root.cmd = cmd
	return root
}

func removeCmdRun(args []string, opts removeOptions) error {
	box := trash.NewBox(
		trash.WithAscend(true),
		trash.WithQueries(args),
		trash.WithQueryMode(trash.ModeByFull),
	)
	if err := box.Open(); err != nil {
		return err
	}

	listFiles(box.Files, false, false)

	for _, arg := range args {
		if box.HitByPath(arg) == 0 {
			glog.Errorf("cannot remove %q: not found in trashcan\n", arg)
		}
	}
	fmt.Printf("\nFound %d trashed files\n", len(box.Files))

	if isTerminal && !tui.BoolPrompt("Are you sure you want to remove PERMENANTLY? ") {
		return errors.New("do nothing")
	}

	if err := doRemove(box.Files); err != nil {
		return err
	}

	return nil
}

func doRemove(files []trash.File) error {
	var failed []trash.File

	for _, file := range files {
		slog.Debug("removing a trashed file", "path", file.TrashPath)
		if err := os.RemoveAll(file.TrashPath); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				glog.Errorf("cannot remove %q: remove: %s\n", file.TrashPath, err)
				failed = append(failed, file)
				continue
			}
		}
		if err := file.Delete(); err != nil {
			// already read, so it is usually not reached
			slog.Warn("removed trashed file but cannot delete .trashinfo", "deletedFile", file.TrashPath, "trashInfoPath", file.TrashInfoPath, "error", err)
		}
	}

	fmt.Printf("Removed %d/%d trashed files\n", len(files)-len(failed), len(files))
	if len(failed) > 0 {
		fmt.Printf("Following %d files could not be deleted.\n", len(failed))
		listFiles(failed, false, true)
	}

	return nil
}
