package xdg

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/maps"
)

type DirCache map[string]*struct {
	Item DirCacheItem
	Seen bool
}

type DirCacheItem struct {
	Size    int64
	Mtime   time.Time
	DirName string
}

func NewDirCache(r io.Reader) (DirCache, error) {
	scan := bufio.NewScanner(r)

	dirCache := make(DirCache)

	for scan.Scan() {
		line := scan.Text()

		parseErr := fmt.Errorf("parse line: %s", line)

		cols := strings.SplitN(line, " ", 3)
		if len(cols) != 3 {
			return nil, parseErr
		}

		size, err := strconv.ParseInt(cols[0], 10, 64)
		if err != nil {
			return nil, parseErr
		}

		ts, err := strconv.ParseInt(cols[1], 10, 64)
		if err != nil {
			return nil, parseErr
		}

		folder, err := url.QueryUnescape(cols[2])
		if err != nil {
			return nil, parseErr
		}

		dirCache[folder] = &struct {
			Item DirCacheItem
			Seen bool
		}{
			Item: DirCacheItem{
				Size:    size,
				Mtime:   time.Unix(ts, 0),
				DirName: folder,
			},
		}
	}

	return dirCache, nil
}

func (i DirCacheItem) String() string {
	return fmt.Sprintf("%d %d %s\n", i.Size, i.Mtime.Unix(), queryEscapePath(i.DirName))
}

func (c DirCache) ToFile(truncate bool) string {
	dirs := maps.Keys(c)
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i] < dirs[j]
	})

	var s strings.Builder
	for _, d := range dirs {
		// remove unseen cache entry
		if truncate && !c[d].Seen {
			continue
		}
		s.WriteString(c[d].Item.String())
	}

	return s.String()
}

func (c DirCache) Save(trashDir string, truncate bool) error {
	// To update the directorysizes file, implementations MUST use a temporary
	// file followed by an atomic rename() operation, in order to avoid
	// corruption due to two implementations writing to the file at the same
	// time.
	f, err := os.CreateTemp("", "directorysizes_gtrash_")
	if err != nil {
		return err
	}
	defer f.Close()
	defer os.Remove(f.Name())

	if _, err = f.WriteString(c.ToFile(truncate)); err != nil {
		return err
	}

	cachePath := filepath.Join(trashDir, "directorysizes")
	if err := os.Rename(f.Name(), cachePath); err != nil {
		// External trash will definitely cause cross-device link errors.
		// so copied trash directory, then rename(2)
		tmpDstPath := filepath.Join(trashDir, filepath.Base(f.Name()))
		dst, err := os.Create(tmpDstPath)
		if err != nil {
			return err
		}
		defer dst.Close()
		defer os.Remove(tmpDstPath)

		// to copy from start, set offset to 0
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		// file copy
		if _, err := io.Copy(dst, f); err != nil {
			return err
		}

		// then rename atomically
		if err := os.Rename(tmpDstPath, cachePath); err != nil {
			return err
		}
	}

	return nil
}
