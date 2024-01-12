package cmd

import (
	"errors"
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/umlx5h/gtrash/internal/glog"
	"github.com/umlx5h/gtrash/internal/trash"
	"github.com/umlx5h/gtrash/internal/tui"
)

type pruneCmd struct {
	cmd  *cobra.Command
	opts pruneOptions
}

type pruneOptions struct {
	force bool

	day  int
	size string // human size (e.g. 10MB, 1G)

	maxTotalSize uint64 // byte, parse from size

	trashDir string // $HOME/.local/share/Trash
}

func (o *pruneOptions) check() error {
	if o.size != "" {
		byte, err := humanize.ParseBytes(o.size)
		if err != nil {
			return fmt.Errorf("--size unit is invalid: %w", err)
		}
		o.maxTotalSize = byte
	}
	return nil
}

func newPruneCmd() *pruneCmd {
	root := &pruneCmd{}
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune trash cans by day or size",
		Long: `Description:
  Pruning trash cans by day or size criteria.
  Either the --day or --size option is required.

  This command is also intended for use via cron.
  By default, you may be prompted multiple times for each trash can.

  If the file to be pruned does not exist, the program exits normally without doing anything.`,
		Example: `  # Delete all files deleted a week ago
  $ gtrash prune --day 7

  # Delete all files deleted a week ago only within $HOME trash
  $ gtrash prune --day 7 --trash-dir "$HOME/.local/share/Trash"

  # Delete files in order from the largest to the smaller one so that the total size of the trash can is less than 5GB.
  # This is useful when you want to keep as many files as possible, including old files, but want to reduce the size of the trash can below a certain level.
  $ gtrash prune --size 5GB

  # Delete large files first to keep the total remaining size under 5GB, while excluding files deleted in the last week.
  # Note that adding the most recently deleted files may exceed 5GB.
  $ gtrash prune --size 5GB --day 7`,
		SilenceUsage:      true,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := pruneCmdRun(root.opts); err != nil {
				return err
			}
			if glog.ExitCode() > 0 {
				return errContinue
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&root.opts.size, "size", "", `Remove files in order from the largest to the smaller one so that the overall size of the trash can is less than the specified size.
If the total size of the trash can is smaller than the specified size, nothing is done.
The total size is calculated by each trash can.

If you want to delete files larger than the specified size, use the "find --size-large XX --rm" command.

Can be specified in human format (e.g. 5MB, 1GB)

If --day and --size are specified at the same time, the most recent X days are excluded from the calculation.
This may be useful when you do not want to delete large files that have been recently deleted.
`)
	cmd.Flags().IntVar(&root.opts.day, "day", 0, "Remove all files deleted before X days")

	cmd.Flags().BoolVarP(&root.opts.force, "force", "f", false, `Always execute without confirmation prompt
This is not necessary if running outside of a terminal
`)
	cmd.Flags().StringVar(&root.opts.trashDir, "trash-dir", "", `Specify a full path if you want to prune only a specific trash can
By default, all trash cans are pruned.

For $HOME trash only:
    --trash-dir "$HOME/.local/share/Trash"
`)
	cmd.Root().MarkFlagsOneRequired("size", "day")

	root.cmd = cmd
	return root
}

// Returns files to be deleted from files based on maxTotalSize
// If maxTotalSize > total, nil is returned.
//
// Prerequisite: files are sorted in ascending order by size
func getPruneFiles(files []trash.File, maxTotalSize uint64) (prune []trash.File, deleted uint64, total uint64) {
	for i, f := range files {
		// If the size cannot be obtained, it is treated as a minus value and should be at the top.
		// This is always skipped and is not considered for deletion.
		if f.Size == nil {
			continue
		}

		size := uint64(*f.Size)
		total += size

		if prune == nil {
			if total > maxTotalSize {
				prune = files[i:]
			}
		}

		if prune != nil {
			deleted += size
		}
	}

	if prune == nil {
		return nil, 0, total
	} else {
		return prune, deleted, total
	}
}

func pruneCmdRun(opts pruneOptions) error {
	if err := opts.check(); err != nil {
		return err
	}

	sortMethod := trash.SortByDeletedAt

	sizeMode := opts.size != ""

	if opts.size != "" {
		sortMethod = trash.SortBySize
	}

	box := trash.NewBox(
		trash.WithSortBy(sortMethod),
		trash.WithGetSize(sizeMode),
		trash.WithAscend(true),
		trash.WithDay(0, opts.day),
		trash.WithTrashDir(opts.trashDir),
	)
	if err := box.Open(); err != nil {
		if errors.Is(err, trash.ErrNotFound) {
			fmt.Printf("do nothing: %s\n", err)
			return nil
		} else {
			return err
		}
	}

	for i, trashDir := range box.TrashDirs {
		files := box.FilesByTrashDir[trashDir]
		if len(files) == 0 {
			continue
		}

		var deleted, total uint64

		if sizeMode {
			files, deleted, total = getPruneFiles(files, opts.maxTotalSize)
			if len(files) == 0 {
				fmt.Printf("do nothing: trash size %s is smaller than %s (%s) in %s\n", humanize.Bytes(total), humanize.Bytes(opts.maxTotalSize), opts.size, trashDir)
				continue
			}
		}

		listFiles(files, sizeMode, false)

		fmt.Printf("\nSelected %d files in %s\n", len(files), trashDir)

		if sizeMode {
			fmt.Printf("Current: %s, Deleted: %s, After: %s, Specified: %s\n\n", humanize.Bytes(total), humanize.Bytes(deleted), humanize.Bytes(total-deleted), humanize.Bytes(opts.maxTotalSize))
		}

		if !opts.force && isTerminal && !tui.BoolPrompt("Are you sure you want to remove PERMENANTLY? ") {
			return errors.New("do nothing")
		}
		doRemove(files)

		if i != len(box.TrashDirs)-1 {
			fmt.Println("")
		}
	}

	return nil
}
