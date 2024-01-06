package posix

import (
	"os"
	"path/filepath"
	"strings"
)

var euid int

func init() {
	euid = os.Geteuid()
}

func AbsPathToTilde(absPath string) string {
	// if executed as root, disable
	if euid == 0 {
		return absPath
	}
	homeDir, ok := os.LookupEnv("HOME")
	if !ok {
		return absPath
	}

	if strings.HasPrefix(absPath, homeDir) {
		return strings.Replace(absPath, homeDir, "~", 1)
	}

	return absPath
}

// Check if sub is a subdirectory of parent
func CheckSubPath(parent, sub string) (bool, error) {
	up := ".." + string(os.PathSeparator)

	rel, err := filepath.Rel(parent, sub)
	if err != nil {
		return false, err
	}
	if !strings.HasPrefix(rel, up) && rel != ".." {
		return true, nil
	}
	return false, nil
}
