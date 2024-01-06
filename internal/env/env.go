package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	// Copy files to the trash can in the home directory when they cannot be renamed to the external trash can
	// Disk usage of the main file system will increase because files are copied across different filesystems, also also take time to copy.
	// Automatically enabled if ONLY_HOME_TRASH enabled
	// Default: false
	HOME_TRASH_FALLBACK_COPY bool

	// Use only the trash can in the home directory, not the one in the external file system
	// Default: false
	ONLY_HOME_TRASH bool

	// Specify the directory for home trash can
	// Default: $XDG_DATA_HOME/Trash ($HOME/.local/share/Trash)
	HOME_TRASH_DIR string

	// Whether to get as close to rm behavior as possible
	// Default: false
	PUT_RM_MODE bool
)

func init() {
	if e, ok := os.LookupEnv("GTRASH_HOME_TRASH_FALLBACK_COPY"); ok {
		if strings.ToLower(strings.TrimSpace(e)) == "true" {
			HOME_TRASH_FALLBACK_COPY = true
		}
	}

	if e, ok := os.LookupEnv("GTRASH_ONLY_HOME_TRASH"); ok {
		if strings.ToLower(strings.TrimSpace(e)) == "true" {
			ONLY_HOME_TRASH = true
			// Also enable this
			HOME_TRASH_FALLBACK_COPY = true
		}
	}

	if e, ok := os.LookupEnv("GTRASH_PUT_RM_MODE"); ok {
		if strings.ToLower(strings.TrimSpace(e)) == "true" {
			PUT_RM_MODE = true
		}
	}

	if e, ok := os.LookupEnv("GTRASH_HOME_TRASH_DIR"); ok {
		if e != "" {
			path, err := filepath.Abs(e)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ENV $GTRASH_HOME_TRASH_DIR is not valid path: %s", err)
				os.Exit(1)
			}

			// Ensure to have directory in advance
			if err := os.MkdirAll(path, 0o700); err != nil {
				fmt.Fprintf(os.Stderr, "ENV $GTRASH_HOME_TRASH_DIR coult not be created: %s", err)
				os.Exit(1)
			}

			HOME_TRASH_DIR = path
		}
	}
}
