package xdg

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	trashHeader = `[Trash Info]`
	timeFormat  = "2006-01-02T15:04:05"
)

// XDG specifications
// https://specifications.freedesktop.org/trash-spec/trashspec-latest.html
// https://specifications.freedesktop.org/desktop-entry-spec/latest/ar01s03.html

type Info struct {
	Path         string    // $PWD/file.go (url decoded)
	DeletionDate time.Time // 2023-01-01T00:00:00
}

func NewInfo(r io.Reader) (Info, error) {
	scanner := bufio.NewScanner(r)

	var info Info

	var (
		groupFound bool
		pathFound  bool
		dateFound  bool
	)
	for scanner.Scan() {
		line := scanner.Text()

		if line == trashHeader {
			groupFound = true
			continue
		}
		if len(line) > 0 && line[0] == '[' {
			// other group found, so exit
			break
		}
		if strings.Contains(line, "=") {
			kv := strings.SplitN(line, "=", 2)

			switch strings.TrimSpace(kv[0]) {
			case "Path":
				if pathFound {
					continue
				}
				u, err := url.QueryUnescape(strings.TrimSpace(kv[1]))
				if err != nil {
					break
				}
				info.Path = u
				pathFound = true
			case "DeletionDate":
				if dateFound {
					continue
				}
				parsed, err := time.ParseInLocation(timeFormat, strings.TrimSpace(kv[1]), time.Local)
				if err != nil {
					break
				}
				info.DeletionDate = parsed
				dateFound = true
			}
		}
	}

	if scanner.Err() != nil {
		return Info{}, scanner.Err()
	}

	if !groupFound || !pathFound || !dateFound {
		return Info{}, errors.New("unable to parse trashinfo")
	}

	return info, nil
}

// represent INI format
func (i Info) String() string {
	return fmt.Sprintf("%s\nPath=%s\nDeletionDate=%s\n", trashHeader, queryEscapePath(i.Path), i.DeletionDate.Format(timeFormat))
}

func (i Info) Save(trashDir TrashDir, filename string) (saveName string, deleteFn func() error, err error) {
	revision := 1

	var trashinfoFile *os.File
	saveName = filename

	for {
		if revision > 1 {
			saveName = fmt.Sprintf("%s_%d", filename, revision)
		}

		// Considering files for which there is no associated trashinfo, check for duplicates under the files directory
		// Since the trashed file may be overwritten by subsequent rename(2)
		if _, err := os.Lstat(filepath.Join(trashDir.FilesDir(), saveName)); err == nil {
			revision++
			continue
		}

		// create .trashinfo file atomically using O_EXCL
		f, err := os.OpenFile(filepath.Join(trashDir.InfoDir(), saveName+".trashinfo"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			// conflict detected, so change to another name
			if errors.Is(err, fs.ErrExist) {
				revision++
				continue
			} else {
				return "", nil, fmt.Errorf("open failed: %w", err)
			}
		}
		defer f.Close()

		trashinfoFile = f
		break
	}

	// Have this called when the file fails to move.
	deleteFn = func() error {
		return os.Remove(trashinfoFile.Name())
	}

	if _, err := trashinfoFile.WriteString(i.String()); err != nil {
		_ = deleteFn()
		return "", nil, fmt.Errorf("write failed: %w", err)
	}

	return saveName, deleteFn, nil
}

// Do not escape '/'
// Escape ' ' as '%20', not '+'
func queryEscapePath(s string) string {
	// do not escape '/'
	a := strings.Split(s, "/")
	for i := 0; i < len(a); i++ {
		// escape ' ' as %20 instead of '+'
		b := strings.Split(a[i], " ")
		for j := 0; j < len(b); j++ {
			b[j] = url.QueryEscape(b[j])
		}
		a[i] = strings.Join(b, "%20")
	}
	return strings.Join(a, "/")
}
