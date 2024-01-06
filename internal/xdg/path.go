package xdg

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/umlx5h/gtrash/internal/env"
)

var (
	// $HOME
	dirHome string
	// $XDG_DATA_HOME
	dirDataHome string

	DirHomeTrash string
)

func init() {
	dirHome = os.Getenv("HOME")
	if dirHome == "" {
		// fallback to get home dir
		u, err := user.Current()
		if err == nil {
			dirHome = u.HomeDir
		}
	}

	dirDataHome = filepath.Join(dirHome, ".local", "share")
	if d, ok := os.LookupEnv("XDG_DATA_HOME"); ok {
		if abs, err := filepath.Abs(d); err == nil {
			dirDataHome = abs
		}
	}

	// Can be changed by environment variables
	if env.HOME_TRASH_DIR != "" {
		DirHomeTrash = env.HOME_TRASH_DIR
	} else {
		DirHomeTrash = filepath.Join(dirDataHome, "Trash")
	}
}
