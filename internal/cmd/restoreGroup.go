package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/umlx5h/gtrash/internal/glog"
	"github.com/umlx5h/gtrash/internal/trash"
	"github.com/umlx5h/gtrash/internal/tui"
)

type restoreGroupCmd struct {
	cmd  *cobra.Command
	opts restoreGroupOptions
}

type restoreGroupOptions struct{}

func newRestoreGroupCmd() *restoreGroupCmd {
	root := &restoreGroupCmd{}

	cmd := &cobra.Command{
		Use:     "restore-group",
		Aliases: []string{"rg"},
		Short:   "Restore trashed files as group interactively (rg)",
		Long: `Description:
  Restore files using TUI.
  Unlike restore, files deleted at the same time are grouped together.

  Multiple selections are not allowed.

  Actually, the files deleted by put are not grouped correctly.
  Files with matching deletion times in seconds are grouped together.

  See below for details.
  ref: https://github.com/umlx5h/gtrash#how-does-the-restore-group-subcommand-work
`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := restoreGroupCmdRun(args, root.opts); err != nil {
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

func restoreGroupCmdRun(args []string, opts restoreGroupOptions) error {
	box := trash.NewBox()
	if err := box.Open(); err != nil {
		return err
	}

	groups := box.ToGroups()

	group, err := tui.GroupSelect(groups)
	if err != nil {
		return err
	}

	listFiles(group.Files, false, false)
	fmt.Printf("\nSelected %d trashed files\n", len(group.Files))

	if isTerminal && !tui.BoolPrompt("Are you sure you want to restore? ") {
		return errors.New("do nothing")
	}

	if err := doRestore(group.Files, ""); err != nil {
		return err
	}

	return nil
}
