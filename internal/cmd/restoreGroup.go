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
		Short:   "Restore trashed files as a group interactively (rg)",
		Long: `Description:
  Use the TUI interface for file restoration.
  Unlike the 'restore' command, files deleted simultaneously are grouped together.

  Multiple selections of groups are not allowed.

  Actually, files deleted using 'gtrash put' may not be grouped accurately.
  Files with deletion times matching in seconds are grouped together.

  Refer below for detailed information.
  ref: https://github.com/umlx5h/gtrash#how-does-the-restore-group-subcommand-work
`,
		SilenceUsage:      true,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := restoreGroupCmdRun(root.opts); err != nil {
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

func restoreGroupCmdRun(_ restoreGroupOptions) error {
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

	if err := doRestore(group.Files, "", true); err != nil {
		return err
	}

	return nil
}
