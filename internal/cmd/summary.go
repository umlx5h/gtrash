package cmd

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/umlx5h/gtrash/internal/glog"
	"github.com/umlx5h/gtrash/internal/trash"
)

type summaryCmd struct {
	cmd  *cobra.Command
	opts summaryOptions
}

type summaryOptions struct{}

func newSummaryCmd() *summaryCmd {
	root := &summaryCmd{}
	cmd := &cobra.Command{
		Use:     "summary",
		Short:   "Show summary of all trash cans (s)",
		Aliases: []string{"s"},
		Long: `Description:
  Displays statistics summarizing all trash cans.
  Shows the count of files (and folders) and their total size.
  When multiple trash cans are detected, the statistics for each and the total are displayed.`,
		SilenceUsage:      true,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := summaryCmdRun(root.opts); err != nil {
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

func summaryCmdRun(opts summaryOptions) error {
	box := trash.NewBox(
		trash.WithGetSize(true),
	)

	if err := box.Open(); err != nil {
		return err
	}

	var (
		totalSize int64
		totalItem int
	)

	for i, trashDir := range box.TrashDirs {
		var (
			size int64
			item int
		)

		for _, f := range box.FilesByTrashDir[trashDir] {
			item++
			if f.Size != nil {
				size += *f.Size
			}
		}

		fmt.Printf("[%s]\n", trashDir)
		fmt.Printf("item: %d\n", item)
		fmt.Printf("size: %s\n", humanize.Bytes(uint64(size)))

		if i != len(box.TrashDirs)-1 {
			fmt.Println("")
		}

		totalSize += size
		totalItem += item
	}

	if len(box.FilesByTrashDir) > 1 {
		fmt.Printf("\n[total]\n")
		fmt.Printf("item: %d\n", totalItem)
		fmt.Printf("size: %s\n", humanize.Bytes(uint64(totalSize)))
	}

	return nil
}
