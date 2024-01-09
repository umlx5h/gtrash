package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	cp "github.com/otiai10/copy"
	"github.com/spf13/cobra"
	"github.com/umlx5h/gtrash/internal/env"
	"github.com/umlx5h/gtrash/internal/glog"
	"github.com/umlx5h/gtrash/internal/posix"
	"github.com/umlx5h/gtrash/internal/tui"
	"github.com/umlx5h/gtrash/internal/xdg"
)

type putCmd struct {
	cmd  *cobra.Command
	opts putOptions
}

type putOptions struct {
	prompt     bool
	promptOnce bool
	force      bool
	verbose    bool

	rmMode    bool
	recursive bool
	dir       bool

	homeFallback bool
}

func newPutCmd() *putCmd {
	root := &putCmd{}

	cmd := &cobra.Command{
		Use:     "put PATH...",
		Aliases: []string{"p"},
		Short:   "Put files to trash (p)",
		Long: `Description:
  A substitute for 'rm', moving files to the trash.
  For files in the main file system, they're moved to the following folder:
      $XDG_DATA_HOME/Trash ($HOME/.local/share/Trash)

  For files in external file systems, they're moved to either of the following locations at the root of the mount point:
      1. $MOUNTPOINT/.Trash/$uid
      2. $MOUNTPOINT/.Trash-$uid

  Folder 1 takes precedence but requires pre-creation with a set sticky bit ($uid part is created automatically).
  Folder 2 is created automatically.

  To identify the folder where files will be moved, use the -v or --debug option.
  To display the path in the trash can, use --show-trashpath with the find command:
      $ gtrash find --show-trashpath

  By default, the options -d, -r, -R, and --recursive are ignored.
  They are unnecessary for file removal but required when using --rm-mode.`,

		Example: `  # -r is unnecessary to delete a folder
  $ gtrash put file1 file2 dir1/ dir2

  # For files starting with a hyphen, specify the filename after the '--'
  $ gtrash put -- -foo

  # If expanded in the shell, you can use glob patterns
  $ gtrash put foo*`,
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := putCmdRun(args, root.opts); err != nil {
				return err
			}
			if glog.ExitCode() > 0 {
				return errContinue
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&root.opts.force, "force", "f", false, "ignore nonexistent files and arguments")
	cmd.Flags().BoolVarP(&root.opts.prompt, "interactive", "i", false, "prompt before every removal")
	// short only options are not available
	cmd.Flags().BoolVarP(&root.opts.promptOnce, "interactive-once", "I", false, "prompt once before trashing")
	cmd.Flags().BoolVarP(&root.opts.verbose, "verbose", "v", false, "explain what is being done")

	// rm mode options if --rm-mode used
	cmd.Flags().BoolVar(&root.opts.rmMode, "rm-mode", env.PUT_RM_MODE, "enable rm-like mode (change behavior -r, -R, -d)")
	cmd.Flags().BoolVarP(&root.opts.dir, "dir", "d", false, `ignored unless --rm-mode set
remove empty directories (--rm-mode)`)
	cmd.Flags().BoolVarP(&root.opts.recursive, "recursive", "r", false, `ignored unless --rm-mode set
remove directories and their contents recursively (--rm-mode)`)

	// TODO: Since short only options are not available, have no choice but to assign a long name.
	cmd.Flags().BoolVarP(&root.opts.recursive, "Recursive", "R", false, "same as -r")

	cmd.Flags().BoolVar(&root.opts.homeFallback, "home-fallback", env.HOME_TRASH_FALLBACK_COPY, `Enable fallback to home directory trash
If the deletion of a file in an external file system fails, this option may help.`)

	root.cmd = cmd
	return root
}

func putCmdRun(args []string, opts putOptions) error {
	if isDebug {
		opts.verbose = true
	}
	if opts.force {
		// If both are specified, force is preferred.
		opts.prompt = false
		opts.promptOnce = false
	}

	slog.Debug("starting put", "args", args, "home-fallback", opts.homeFallback, "rm-mode", opts.rmMode)

	if (opts.prompt || opts.promptOnce) && !isTerminal {
		return errors.New("cannot use -i without tty")
	}

	if opts.promptOnce {
		// -I confirmation dialog
		for _, a := range args {
			fmt.Println(a)
		}

		fmt.Println("")
		yes := tui.BoolPrompt(fmt.Sprintf("Do you trash above %d items? ", len(args)))
		if !yes {
			return errors.New("canceled")
		}
	}

	// could restore-group to work, reuse deleteTime
	var deleteTime time.Time

	for _, arg := range args {
		// same as rm
		if slices.Contains([]string{".", ".."}, filepath.Base(arg)) {
			glog.Errorf("refusing to remove '.' or '..' directory: skipping %q\n", arg)
			continue
		}

		slog.Debug("checking for the existence of files with lstat(2)", "file", arg)
		st, err := os.Lstat(arg)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				if !opts.force {
					glog.Errorf("cannot remove %q: No such file or directory\n", arg)
				}
			} else {
				glog.Errorf("cannot remove %q: %s\n", err)
			}
			continue
		}

		if opts.rmMode {
			if st.IsDir() {
				if !opts.recursive && !opts.dir {
					glog.Errorf("cannot remove %q: Is a directory\n", arg)
					continue
				}

				if !opts.recursive && opts.dir {
					// check if directory is empty
					empty, err := posix.DirEmpty(arg)
					if err != nil {
						glog.Errorf("cannot remove %q: check dir empty: %s\n", arg, err)
						continue
					}

					if !empty {
						glog.Errorf("cannot remove %q: Directory not empty\n", arg)
						continue
					}
				}
			}
		}

		// -i confirmation dialog
		if opts.prompt {
			prompt := fmt.Sprintf("Do you trash %s %q? ", posix.FileType(st), arg)
			choices := []string{"yes", "no", "all-yes", "quit"}
			selected, err := tui.ChoicePrompt(prompt, choices, nil)
			if err != nil {
				// canceled
				return err
			}
			switch selected {
			case "no":
				continue // skip
			case "all-yes":
				// disable prompt later
				opts.prompt = false
			}
		}

		path, err := filepath.Abs(arg)
		if err != nil {
			glog.Errorf("cannot remove %q: get abspath: %s\n", arg, err)
			continue
		}

		// for -v logging
		var usedDir xdg.TrashDir

		slog.Debug("looking up trash_dir", "path", path)

		// TODO: Add integration test
		homeDir, externalDir, err := xdg.LookupTrashDir(path)

		slog.Debug("looked up trash_dir", "homeDir", homeDir, "externalDir", externalDir, "error", err)

		if err != nil {
			if !opts.homeFallback || (homeDir == nil && externalDir == nil) {
				glog.Errorf("cannot remove %q: lookup trash directory: %s\n", arg, err)
				continue
			}

			// fallback to home trash
			slog.Debug("fallback to home trash because external trash is not found", "error", err)
		}

		// preferred if an external trash can is available.
		if externalDir != nil {
			slog.Debug("will use external trash, will use rename(2) to move", "trashDir", externalDir.Dir)
			// external trash only uses rename, not copy
			if err := trashFile(*externalDir, path, &deleteTime, false); err != nil {
				if !opts.homeFallback {
					glog.Errorf("cannot remove %q: %s\n", arg, err)
					continue
				}

				// fallback to home trash
				slog.Debug("fallback to home trash because moving failed by rename(2)", "error", err)
			} else {
				usedDir = *externalDir
				goto SUCCESS
			}
		}

		if opts.homeFallback || env.ONLY_HOME_TRASH {
			slog.Debug("will use home trash, will use rename(2) and copy to move", "trashDir", homeDir.Dir)
		} else {
			slog.Debug("will use home trash, will use rename(2) to move", "trashDir", homeDir.Dir)
		}
		if err := trashFile(*homeDir, path, &deleteTime, opts.homeFallback || env.ONLY_HOME_TRASH); err != nil {
			glog.Errorf("cannot remove %q: %s\n", arg, err)
			continue
		}
		usedDir = *homeDir

	SUCCESS:
		if opts.verbose {
			fmt.Printf("trashed %q to %s\n", arg, posix.AbsPathToTilde(usedDir.Dir))
		}
	}
	return nil
}

func trashFile(trashDir xdg.TrashDir, path string, deleteTime *time.Time, fallbackCopy bool) error {
	if err := trashDir.CreateDir(); err != nil {
		return fmt.Errorf("create trash directory: %w\n", err)
	}

	infoPath := path
	if trashDir.UseRelativePath() {
		// get relative path from $topDir
		if p, err := filepath.Rel(trashDir.Root, path); err == nil {
			// it MUST not include a “..” directory, and for files not “under” that directory, absolute pathnames must be used
			if p != ".." && !strings.HasPrefix(p, ".."+string(os.PathSeparator)) {
				infoPath = p
			}
		} else {
			// should not come here
			slog.Warn("cannot convert absolute to relative path, use absolute path instead", "file", path, "root", trashDir.Root, "error", err)
		}
	}

	if deleteTime.IsZero() {
		*deleteTime = time.Now()
	}

	info := xdg.Info{
		Path:         infoPath,
		DeletionDate: *deleteTime,
	}

	filename := filepath.Base(path)
	// before rename(2), write .trashinfo metadata atomically
	saveName, deleteFn, err := info.Save(trashDir, filename)
	if err != nil {
		return fmt.Errorf("save trashinfo: %w\n", err)
	}

	slog.Debug("saved .trashinfo metadata", "path", filepath.Join(trashDir.InfoDir(), saveName+".trashinfo"))

	// move file to trash
	dstPath := filepath.Join(trashDir.FilesDir(), saveName)

	slog.Debug("executing rename(2) to move", "from", path, "to", dstPath)
	if err := os.Rename(path, dstPath); err != nil {
		if fallbackCopy {
			// rename(2) failed, fallback to copy and delete
			slog.Debug("executing copy and delete to move because rename(2) failed", "from", path, "to", dstPath, "error", err)
			// copy recursively
			if err := cp.Copy(path, dstPath); err != nil {
				_ = deleteFn()
				return fmt.Errorf("fallback copy: %w", err)
			}

			// if copy success, then remove recursively
			if err = os.RemoveAll(path); err != nil {
				_ = deleteFn()
				return fmt.Errorf("delete after fallback copy: %w", err)
			}

			return nil
		}

		// delete corrensponding .trashinfo file
		_ = deleteFn()

		return fmt.Errorf("move: %w", err)
	}

	return nil
}
