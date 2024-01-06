package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/umlx5h/gtrash/internal/glog"
	"github.com/umlx5h/gtrash/internal/trash"
	"github.com/umlx5h/gtrash/internal/tui"
)

type metafixCmd struct {
	cmd  *cobra.Command
	opts metafixOptions
}

type metafixOptions struct{}

func newMetafixCmd() *metafixCmd {
	root := &metafixCmd{}
	cmd := &cobra.Command{
		Use:   "metafix",
		Short: "Fix trashcan metadata",
		Long: `Description:
  Detect and delete meta informations for which no corresponding files exist.

  This is useful after manually deleting files in the Trash directory.
  See below for details.

  https://github.com/umlx5h/gtrash#what-does-the-metafix-subcommand-do`,
		SilenceUsage:      true,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := metafixCmdRun(root.opts); err != nil {
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

func metafixCmdRun(opts metafixOptions) error {
	box := trash.NewBox(
		trash.WithSortBy(trash.SortByName),
	)
	if err := box.Open(); err != nil {
		return err
	}

	if len(box.OrphanMeta) == 0 {
		fmt.Println("Not found invalid metadata")
		return nil
	}

	listFiles(box.OrphanMeta, false, false)

	// TODO: Add functionality to allow deletion of orphaned files as well
	// (those for which trashinfo exists but the file does not).
	fmt.Printf("\nFound invalid metadata: %d\n", len(box.OrphanMeta))

	if isTerminal && !tui.BoolPrompt("Are you sure you want to remove invalid metadata? ") {
		return errors.New("do nothing")
	}

	var failed int
	for _, f := range box.OrphanMeta {
		if err := os.Remove(f.TrashInfoPath); err != nil {
			failed++
			glog.Errorf("cannot remove .trashinfo: %q: %s\n", f.TrashInfoPath, err)
		}
	}

	fmt.Printf("Deleted invalid metadata: %d\n", len(box.OrphanMeta)-failed)

	return nil
}
