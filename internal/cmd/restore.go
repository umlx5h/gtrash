package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	cp "github.com/otiai10/copy"
	"github.com/rs/xid"
	"github.com/spf13/cobra"
	"github.com/umlx5h/gtrash/internal/glog"
	"github.com/umlx5h/gtrash/internal/trash"
	"github.com/umlx5h/gtrash/internal/tui"
)

type restoreCmd struct {
	cmd  *cobra.Command
	opts restoreOptions
}

type restoreOptions struct {
	directory string
	cwd       bool
	restoreTo string
	force     bool
}

func newRestoreCmd() *restoreCmd {
	root := &restoreCmd{}

	cmd := &cobra.Command{
		Use:     "restore [PATH...]",
		Aliases: []string{"r"},
		Short:   "Restore trashed files interactively (r)",
		Long: `Description:
  Use the TUI interface to restore files, enabling multiple file selection.
  Press the ? key within the TUI interface for usage help.

  When specifying the full path in the command-line argument, restoration is performed without using the TUI interface.`,
		Example: `  # Restore interactively
  $ gtrash restore

  # Restore files without TUI
  # Must specify full paths
  $ gtrash restore /home/user/file1 /home/user/file2

  # Fuzzy find multiple items and restore them
  # The -o in xargs is necessary for the confirmation prompt to display.
  $ gtrash find | fzf --multi | awk -F'\t' '{print $2}' | xargs -o gtrash restore`,
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if err := restoreCmdRun(args, root.opts); err != nil {
				return err
			}
			if glog.ExitCode() > 0 {
				return errContinue
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&root.opts.directory, "directory", "d", "", "Filter by directory")
	cmd.Flags().BoolVarP(&root.opts.cwd, "cwd", "c", false, "Filter by current working directory")
	cmd.Flags().StringVar(&root.opts.restoreTo, "restore-to", "", "Restore to this path instead of original path")
	cmd.Flags().BoolVarP(&root.opts.force, "force", "f", false, `Always execute without confirmation prompt
This is not necessary if running outside of a terminal`)

	root.cmd = cmd
	return root
}

func restoreCmdRun(args []string, opts restoreOptions) (err error) {
	if err := checkOptRestoreTo(&opts.restoreTo); err != nil {
		return err
	}

	slog.Debug("starting restore", "args", args)

	box := trash.NewBox(
		trash.WithDirectory(opts.directory),
		trash.WithCWD(opts.cwd),
		trash.WithQueries(args),               // only used when specifying command args
		trash.WithQueryMode(trash.ModeByFull), // only support full match
	)
	if err := box.Open(); err != nil {
		return err
	}

	if len(args) == 0 {
		if !isTerminal {
			return errors.New("cannot use tui interface, please specify restore path to command line args")
		}

		// interactive restore when not specifying command line args
		box.Files, err = tui.FilesSelect(box.Files)
		if err != nil {
			return err
		}
	}

	listFiles(box.Files, false, false)

	for _, arg := range args {
		if box.HitByPath(arg) == 0 {
			glog.Errorf("cannot restore %q: not found in trashcan\n", arg)
		}
	}

	fmt.Printf("\nSelected %d trashed files\n", len(box.Files))

	if opts.restoreTo != "" {
		fmt.Printf("Will restore to %q instead of original path\n", opts.restoreTo)
	}

	if !opts.force && isTerminal && !tui.BoolPrompt("Are you sure you want to restore? ") {
		return errors.New("do nothing")
	}

	if err := doRestore(box.Files, opts.restoreTo, isTerminal && !opts.force); err != nil {
		return err
	}

	return nil
}

func checkOptRestoreTo(restoreTo *string) error {
	if restoreTo == nil {
		return nil
	}

	if *restoreTo != "" {
		fi, err := os.Stat(*restoreTo)
		if err != nil {
			return fmt.Errorf("--restore-to path must be existing directory: %w", err)
		}

		if !fi.IsDir() {
			return fmt.Errorf("--restore-to path must be directory")
		}

		// convert to absolute path
		abs, err := filepath.Abs(*restoreTo)
		if err != nil {
			return fmt.Errorf("--restore-to path must be valid directory: %w", err)
		}
		*restoreTo = abs
	}

	return nil
}

func checkRestoreDup(files []trash.File) error {
	// Detect and abort duplicate restore destinations
	fileByPath := make(map[string][]trash.File)

	for _, f := range files {
		fileByPath[f.OriginalPath] = append(fileByPath[f.OriginalPath], f)
	}

	var conflicted bool
	for path, files := range fileByPath {
		if len(files) >= 2 {
			conflicted = true
			glog.Errorf("conflict restore %d files: %q\n", len(files), path)
		}
	}

	if conflicted {
		return errors.New("canceled: restore conflict detected")
	}

	return nil
}

func doRestore(files []trash.File, restoreTo string, prompt bool) error {
	if !prompt {
		if err := checkRestoreDup(files); err != nil {
			return err
		}
	}

	var (
		success int
		failed  []trash.File
	)

	printResult := func() {
		if restoreTo != "" {
			fmt.Printf("Restored to %q\n", restoreTo)
		}

		fmt.Printf("Restored %d/%d trashed files\n", success, len(files))
		if len(failed) > 0 {
			fmt.Printf("Following %d files could not be restored.\n", len(failed))
			listFiles(failed, false, true)
		}
	}

	defer printResult()

	var (
		repeat     bool
		selected   string
		prevSelect string
	)

	for _, file := range files {
		// option to change the restore destination.
		restorePath := file.OriginalPath
		if restoreTo != "" {
			restorePath = filepath.Join(restoreTo, file.OriginalPath)
		}

		// Check to see if the file already exists in the destination path.
		// This is necessary because rename(2) overwrites the file.
		if _, err := os.Lstat(restorePath); err == nil {
			if !prompt {
				glog.Errorf("cannot restore %q: restore path alread exists\n", file.OriginalPath)
				failed = append(failed, file)
				continue
			}
			if !repeat {
				choice := []string{"new-name", "skip", "quit"}
				if prevSelect != "" {
					choice = []string{"new-name", "skip", "repeat-prev", "quit"}
				}
				// TODO: Make the message easy to understand
				selected, err = tui.ChoicePrompt(fmt.Sprintf("Conflicted restore path %q\n\tPlease choose one of the following: ", file.OriginalPath), choice, nil)
				if err != nil {
					return err
				}
			}

		SWITCH:
			switch selected {
			case "new-name":
				// give a random string to avoid duplicates
				restorePath = restorePath + "." + xid.New().String()
				fmt.Printf("Restoring to %q (original: %q)\n", restorePath, file.Name)
				prevSelect = selected
			case "skip":
				prevSelect = selected
				continue
			case "repeat-prev":
				repeat = true
				selected = prevSelect
				goto SWITCH
			}
		}

		// ensure to have directory to restore
		if err := os.MkdirAll(filepath.Dir(restorePath), 0o777); err != nil {
			glog.Errorf("cannot restore %q: mkdir restorePath: %s\n", file.OriginalPath, err)
			failed = append(failed, file)
			continue
		}

		// "overwrite" is not an option because it only works when the source and destination files are both files.
		// old     new
		// file   file      old overwrites new
		//  dir   file      error: not a directory
		// file    dir      error: file exists
		//  dir    dir      error: file exists

		slog.Debug("executing rename(2) to restore", "from", file.TrashPath, "to", restorePath)
		if err := os.Rename(file.TrashPath, restorePath); err != nil {
			// rename(2) failed, fallback to copy and delete
			slog.Debug("executing copy and delete to restore because rename(2) failed", "from", file.TrashPath, "to", restorePath)

			// copy recursively
			if err := cp.Copy(file.TrashPath, restorePath); err != nil {
				glog.Errorf("cannot restore %q: fallback copy: %s\n", file.OriginalPath)
				failed = append(failed, file)
				continue
			}

			// if copy success, then remove recursively
			if err = os.RemoveAll(file.TrashPath); err != nil {
				slog.Warn("restored successfully but cannot delete trashed file", "trashPath", file.TrashPath, "restoreTo", file.OriginalPath, "error", err)
			}
		}

		if err := file.Delete(); err != nil {
			slog.Warn("restored successfully but cannot delete .trashinfo", "trashInfoPath", file.TrashInfoPath, "restoreTo", file.OriginalPath, "error", err)
		}

		success++
	}

	return nil
}
