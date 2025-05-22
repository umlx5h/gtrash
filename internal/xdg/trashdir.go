package xdg

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/moby/sys/mountinfo"
	"github.com/umlx5h/gtrash/internal/env"
)

type trashDirType string

const (
	trashDirTypeHome        trashDirType = "HOME"         // $XDG_DATA_HOME/Trash
	trashDirTypeExternal    trashDirType = "EXTERNAL"     // $root/.Trash/$uid
	trashDirTypeExternalAlt trashDirType = "EXTERNAL_ALT" // $root/.Trash-$uid
	trashDirTypeManual      trashDirType = "MANUAL"       // any directory, specify by --trash-dir
)

type TrashDir struct {
	Root    string // $XDG_DATA_HOME or $rootDir (used for relative path)
	Dir     string // $XDG_DATA_HOME/Trash or $rootDir/.Trash/$uid or $rootDir/.Trash-$uid (has info and files directory)
	dirType trashDirType
}

func (d TrashDir) InfoDir() string {
	return filepath.Join(d.Dir, "info")
}

func (d TrashDir) FilesDir() string {
	return filepath.Join(d.Dir, "files")
}

// Use relative paths for external trash
func (d TrashDir) UseRelativePath() bool {
	switch d.dirType {
	case trashDirTypeHome: // use absolute path
		return false
	case trashDirTypeExternal, trashDirTypeExternalAlt: // use relative path
		return true
	default:
		panic("not reached")
	}
}

func (d TrashDir) CreateDir() error {
	if err := os.MkdirAll(d.InfoDir(), 0o700); err != nil {
		return err
	}

	if err := os.MkdirAll(d.FilesDir(), 0o700); err != nil {
		return err
	}

	return nil
}

func NewTrashDirManual(dir string) TrashDir {
	return TrashDir{
		Root:    filepath.Dir(dir),
		Dir:     dir,
		dirType: trashDirTypeManual,
	}
}

// Scan and returns trash directories from all mountpoints
// The existence of the 'files' and 'info' directories is not checked
func ScanTrashDirs() []TrashDir {
	var trashDirList []TrashDir

	// 1. First get the trash can in the home directory
	if _, err := os.Stat(DirHomeTrash); err == nil {
		trashDirList = append(trashDirList, TrashDir{
			Root:    dirDataHome,
			Dir:     DirHomeTrash,
			dirType: trashDirTypeHome,
		})
		slog.Debug("found home trash", "directory", DirHomeTrash)
	}

	if env.ONLY_HOME_TRASH {
		return trashDirList
	}

	// Get all mount points to get external trash cans
	slog.Debug("getting all mountpoints")
	topDirs, err := getAllMountpoints()
	if err != nil {
		slog.Warn("failed to get all mountpoints, do not use external trash", "error", err)
		return trashDirList
	}

	uid := strconv.Itoa(os.Getuid())

	// Check to see if the .Trash directory exists
	for _, topDir := range topDirs {
		// 2. check $topDir/.Trash/$uid
		trashDir := filepath.Join(topDir, ".Trash", uid)

		if _, err := os.Stat(trashDir); err == nil {
			trashDirList = append(trashDirList, TrashDir{
				Root:    topDir,
				Dir:     trashDir,
				dirType: trashDirTypeExternal,
			})
			slog.Debug("found external trash", "directory", trashDir)
		}

		// 3. check $topDir/Trash-$uid
		trashDir = filepath.Join(topDir, fmt.Sprintf(".Trash-%s", uid))
		if _, err = os.Stat(trashDir); err == nil {
			trashDirList = append(trashDirList, TrashDir{
				Root:    topDir,
				Dir:     trashDir,
				dirType: trashDirTypeExternalAlt,
			})
			slog.Debug("found external alternative trash", "directory", trashDir)
		}
	}

	return trashDirList
}

// Returns the trash directory associated with the file
// Return the home directory for fallback as well.
func LookupTrashDir(path string) (home *TrashDir, external *TrashDir, err error) {
	homeTrash := &TrashDir{
		Root:    dirDataHome,
		Dir:     DirHomeTrash,
		dirType: trashDirTypeHome,
	}

	// always using home trash
	if env.ONLY_HOME_TRASH {
		// already create dir in env.go
		return homeTrash, nil, nil
	}

	// 1. Determine whether to use the trash can in the home directory
	// stat(2) each file and determine that they have the same file system if the device number (st_dev) matches.
	// implementation varies by program.
	sameFS, err := useHomeTrash(path)
	if err != nil {
		// unexpected error
		return nil, nil, fmt.Errorf("home_trash: %w", err)
	}

	if sameFS {
		// use home_trash
		return homeTrash, nil, nil
	}

	// obtain a mount point associated with a file
	topDir, err := getMountpoint(path)
	if err != nil {
		return homeTrash, nil, fmt.Errorf("get mountpoint: %w", err)
	}

	// 2. Check $topDir/.Trash/$uid available
	if trashDir, err := useExternalTrash(topDir); err == nil {
		return homeTrash, &TrashDir{
			Root:    topDir,
			Dir:     trashDir,
			dirType: trashDirTypeExternal,
		}, nil
	}

	// 3. Check $topDir/Trash-$uid available
	if trashDir, err := useExternalTrashAlt(topDir); err == nil {
		return homeTrash, &TrashDir{
			Root:    topDir,
			Dir:     trashDir,
			dirType: trashDirTypeExternalAlt,
		}, nil
	} else {
		return homeTrash, nil, fmt.Errorf("external_trash: %w", err)
	}
}

// Exclude file systems from find that are clearly unnecessary
var skipFSType = []string{
	"binfmt_misc",
	"cgroup",
	"cgroup2",
	"debugfs",
	"devpts",
	"devtmpfs",
	"hugetlbfs",
	"mqueue",
	"proc",
	"sysfs",
	"tracefs",
	"nsfs",
	"fusectl",
}

func getAllMountpoints() ([]string, error) {
	infos, err := mountinfo.GetMounts(func(i *mountinfo.Info) (skip bool, stop bool) {
		if slices.Contains(skipFSType, i.FSType) {
			return true, false
		}

		// Read-only file systems are also excluded.
		if i.Options == "ro" || strings.HasPrefix(i.Options, "ro,") {
			return true, false
		}

		return false, false
	})
	if err != nil {
		return nil, err
	}

	// sometimes, same mountpoint exists, so must take a unique
	mountpoints := make([]string, 0, len(infos))
	exists := make(map[string]struct{}, len(infos))
	for i := range infos {
		m := infos[i].Mountpoint

		if _, ok := exists[m]; ok {
			// duplicate entry detected
			slog.Debug("duplicated mountpoint is detected", "mountpoint", m)
			continue
		}

		mountpoints = append(mountpoints, m)
		exists[m] = struct{}{}
	}

	return mountpoints, nil
}

var mountinfo_Mounted = mountinfo.Mounted
var EvalSymLinks = filepath.EvalSymlinks

// Obtain a mount point associated with a file.
// Same as df <PATH>
func getMountpoint(path string) (string, error) {

	// iterate over the real (without symlinks) parents of path until we find a mount point

	candidate, err := EvalSymLinks(filepath.Dir(path))
	if err != nil {
		return "", err
	}

	for {
		// root is always mounted
		if candidate == string(os.PathSeparator) {
			slog.Debug("root mountpoint is detected", "path", path)
			break
		}

		if candidate == "." {
			// should not reached here
			// check to prevent busy loop
			return "", errors.New("mountpoint is '.'")
		}

		mounted, err := mountinfo_Mounted(candidate)
		if err != nil {
			return "", err
		}

		if mounted {
			break
		}

		candidate = filepath.Dir(candidate)
	}

	return candidate, nil
}

func useHomeTrash(path string) (sameFS bool, err error) {
	// do not follow symlink
	fi, err := os.Lstat(path)
	if err != nil {
		// must be already checked
		return false, err
	}

	ti, err := os.Stat(DirHomeTrash)
	if err != nil {
		// if home trash folder do not exist, create it
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(DirHomeTrash, 0o700); err != nil {
				return false, fmt.Errorf("create trash_dir: %w", err)
			}
			// re-execute stat
			ti, err = os.Stat(DirHomeTrash)
		}
	}
	if err != nil {
		return false, fmt.Errorf("stat(2) trash_dir: %w", err)
	}

	fromInfo, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false, fmt.Errorf("get stat(2) dev_ino")
	}

	toInfo, ok := ti.Sys().(*syscall.Stat_t)
	if !ok {
		return false, fmt.Errorf("get stat(2) dev_ino from trash_dir")
	}

	// stat(2) struct stat { dev_t st_dev }
	if fromInfo.Dev == toInfo.Dev {
		// If the device number matches, the home trash can be used because it is the same file system.
		return true, nil
	}

	// different file system
	return false, nil
}

func useExternalTrash(topDir string) (string, error) {
	// xdg ref: When trashing a file from a non-home partition/device4 , an
	// implementation (if it supports trashing in top directories) MUST
	// check for the presence of $topdir/.Trash.
	trashDir := filepath.Join(topDir, ".Trash")
	info, err := os.Lstat(trashDir)
	if err != nil {
		return "", errors.New(".Trash not found")
	}
	if !info.IsDir() {
		return "", errors.New(".Trash is not directory")
	}

	// xdg ref: The implementation also MUST check that this directory is not a symbolic link.
	if info.Mode().Type() == fs.ModeSymlink {
		return "", errors.New(".Trash is symlink")
	}

	// xdg ref: If this directory is present, the implementation MUST, by default, check for the “sticky bit”.
	if info.Mode()&os.ModeSticky == 0 {
		return "", errors.New(".Trash sticky bit not set")
	}

	trashDir = filepath.Join(trashDir, strconv.Itoa(os.Getuid()))

	// Ensure to have $topDir/$uid directory
	if err := os.MkdirAll(trashDir, 0o700); err != nil {
		return "", fmt.Errorf("%q not created: %w", trashDir, err)
	}

	return trashDir, nil
}

func useExternalTrashAlt(topDir string) (string, error) {
	trashDir := filepath.Join(topDir, fmt.Sprintf(".Trash-%d", os.Getuid()))

	// Ensure to have $topDir-$uid directory
	if err := os.MkdirAll(trashDir, 0o700); err != nil {
		return "", fmt.Errorf("%q not created: %w", trashDir, err)
	}

	return trashDir, nil
}
