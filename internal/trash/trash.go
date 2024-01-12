package trash

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/gobwas/glob"
	"github.com/spf13/pflag"
	"github.com/umlx5h/gtrash/internal/posix"
	"github.com/umlx5h/gtrash/internal/xdg"
)

var _ pflag.Value = (*SortByType)(nil)

type SortByType int

const (
	SortByDeletedAt SortByType = iota // default
	SortBySize
	SortByName
)

type Box struct {
	Files           []File
	FilesByTrashDir map[string][]File // key: trash_dir, value: array of Files
	TrashDirs       []string
	hitByPath       map[string]int // key: originalPath, value: number of files to hit
	OrphanMeta      []File         // .trashinfo exists but there is no real file in the files folder

	// set by cli flags

	// sort options
	ascend bool
	sortBy SortByType

	// filter options
	cwd         bool
	directory   string
	queries     []string
	queriesReg  []*regexp.Regexp
	queriesGlob []glob.Glob
	queryModeBy ModeByType

	// filter by date
	day      int
	dayPoint time.Time // --day-new, --day-old
	newer    bool

	// filter by size
	size       uint64 // byte, convert from sizeHuman
	sizeHuman  string // human size (e.g. 10MB)
	sizeLarger bool   // if true, filter by size > X

	trashDir string // $HOME/.local/share/Trash

	// Whether to use stat(2) to get size and mode
	GetSize       bool
	noFilterApply bool // true if select all trashcan

	limitLast int
}

func NewBox(opts ...BoxOption) Box {
	b := Box{
		FilesByTrashDir: make(map[string][]File),
		hitByPath:       make(map[string]int),
	}

	for _, o := range opts {
		o(&b)
	}

	return b
}

type BoxOption func(*Box)

func WithAscend(ascend bool) BoxOption {
	return func(b *Box) {
		b.ascend = ascend
	}
}

func WithTrashDir(trashDir string) BoxOption {
	return func(b *Box) {
		b.trashDir = trashDir
	}
}

func WithSortBy(sortBy SortByType) BoxOption {
	return func(b *Box) {
		b.sortBy = sortBy
	}
}

func WithDirectory(directory string) BoxOption {
	return func(b *Box) {
		b.directory = directory
	}
}

func WithCWD(cwd bool) BoxOption {
	return func(b *Box) {
		b.cwd = cwd
	}
}

func WithQueries(queries []string) BoxOption {
	return func(b *Box) {
		b.queries = queries
	}
}

func WithQueryMode(mode ModeByType) BoxOption {
	return func(b *Box) {
		b.queryModeBy = mode
	}
}

// TODO: Support for notations other than day?
func WithDay(dayNew int, dayOld int) BoxOption {
	day := max(dayNew, dayOld) // either specified
	point := time.Now().AddDate(0, 0, -day)

	return func(b *Box) {
		b.day = day
		b.dayPoint = point
		b.newer = dayNew > 0
	}
}

func WithLimitLast(last int) BoxOption {
	return func(b *Box) {
		b.limitLast = last
	}
}

func WithSize(large string, small string) BoxOption {
	var larger bool
	size := small
	if large != "" {
		larger = true
		size = large
	}

	return func(b *Box) {
		b.sizeHuman = size // either specified
		b.sizeLarger = larger
	}
}

func WithGetSize(get bool) BoxOption {
	return func(b *Box) {
		b.GetSize = get
	}
}

// validate and adjust options
func (b *Box) checkOptions() error {
	var err error

	// convert to absolute path
	if b.directory != "" {
		if abs, err := filepath.Abs(b.directory); err == nil {
			b.directory = abs
		}
	}

	// get cwd
	if b.cwd {
		b.directory, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("-c,--cwd get cwd: %w", err)
		}
	}

	// compile queries
	if len(b.queries) > 0 {
		switch b.queryModeBy {
		case ModeByRegex:
			regs := make([]*regexp.Regexp, len(b.queries))
			for i, q := range b.queries {
				r, err := regexp.Compile(q)
				if err != nil {
					return fmt.Errorf("regex syntax in query is not valid: %q: %q", q, err)
				}
				regs[i] = r
			}

			b.queriesReg = regs
		case ModeByGlob:
			globs := make([]glob.Glob, len(b.queries))
			for i, q := range b.queries {
				g, err := glob.Compile(q)
				if err != nil {
					return fmt.Errorf("glob syntax in query is not valid: %q: %w", q, err)
				}
				globs[i] = g
			}

			b.queriesGlob = globs
		}
	}

	// compile human-size byte to byte
	if b.sizeHuman != "" {
		byte, err := humanize.ParseBytes(b.sizeHuman)
		if err != nil {
			return fmt.Errorf("--size unit is invalid: %w", err)
		}
		b.size = byte
	}

	// set GetSize true based options
	if !b.GetSize {
		b.GetSize = b.sizeHuman != "" || b.sortBy == SortBySize
	}

	// validate as absolute path and normalize and check existence
	if b.trashDir != "" {
		// validate
		if !filepath.IsAbs(b.trashDir) {
			return fmt.Errorf("--trash-dir is not absolute path")
		}

		// normalize
		b.trashDir, _ = filepath.Abs(b.trashDir)

		// check existence
		if fi, err := os.Stat(b.trashDir); err != nil {
			return fmt.Errorf("--trash-dir must be a existing directory: %w", err)
		} else {
			if !fi.IsDir() {
				return fmt.Errorf("--trash-dir must be a directory")
			}
		}
	}

	// check if select all trashcan
	if len(b.queries) == 0 && b.sizeHuman == "" && b.day == 0 && b.directory == "" {
		b.noFilterApply = true
	}

	return nil
}

var ErrNotFound = errors.New("not found")

func (b *Box) Open() error {
	// validation Box options
	if err := b.checkOptions(); err != nil {
		return err
	}

	var trashDirs []xdg.TrashDir

	if b.trashDir == "" {
		// Automatically searches for trash can paths by default
		slog.Debug("scanning trash directories")
		// Retrieve trash from all mount points
		trashDirs = xdg.ScanTrashDirs()
		if len(trashDirs) == 0 {
			return fmt.Errorf("%w: trash directories", ErrNotFound)
		}
		slog.Debug("found trash directories", "number", len(trashDirs), "trashDirs", trashDirs)
	} else {
		// If --trash-dir is specified, it is used as is.
		slog.Debug("using manual trash directory", "trashDir", b.trashDir)
		trashDirs = []xdg.TrashDir{xdg.NewTrashDirManual(b.trashDir)}
	}

	for _, trashDir := range trashDirs {
		slog.Debug("starting to read trashDir", "trashDir", trashDir.Dir)
		// Scan the files directory to check for the existence of files.
		// Whether the file is a directory or not can be obtained at this stage.
		dirents, err := os.ReadDir(trashDir.FilesDir())
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				slog.Warn("cannot read files folder in trashDir, skipped", "trashDir", trashDir, "error", err)
				continue
			}
			// files folder not exists in this mountpoint
			slog.Debug("not found files folder in trashDir, skipped", "trashDir", trashDir)
			continue
		}
		// convert to slices to map
		fileEntries := make(map[string]bool, len(dirents)) // key: filename, value: isDir
		for _, ent := range dirents {
			fileEntries[ent.Name()] = ent.IsDir()
		}

		dirents, err = os.ReadDir(trashDir.InfoDir())
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				slog.Warn("cannot read info folder in trashDir, skipped", "trashDir", trashDir, "error", err)
				continue
			}

			slog.Debug("not found info folder in trashDir, skipped", "trashDir", trashDir)
			continue
		}

		// Load directory size cache into map
		// Not used when nil.
		var dirCache xdg.DirCache // key: directory name, value: cache entry

		directorySizesPath := filepath.Join(trashDir.Dir, "directorysizes")

		if b.GetSize {
			// init map
			dirCache = make(xdg.DirCache)

			slog.Debug("reading directorysizes cache", "path", directorySizesPath)
			if f, err := os.Open(directorySizesPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					slog.Debug("not found directorysizes cache", "path", directorySizesPath, "error", err)
				} else {
					slog.Warn("failed to read directorysizes cache", "path", directorySizesPath)
				}
			} else {
				if c, err := xdg.NewDirCache(f); err != nil {
					slog.Warn("failed to parse directorysizes cache, it will be recreated", "path", directorySizesPath, "error", err)
				} else {
					// got cache from file
					dirCache = c
				}
				f.Close()
			}
		}

		slog.Debug("starting to read directory entries", "file_entries", len(fileEntries), "info_entries", len(dirents))

		// True if the cache expires or an entry is added.
		var dirCacheUpdated bool

		files := b.getFiles(dirents, fileEntries, trashDir, dirCache, &dirCacheUpdated)
		slog.Debug("found trashed files", "number", len(files), "trashDir", trashDir.Dir)

		// save directorysize cache
		if dirCache != nil && dirCacheUpdated {
			slog.Debug("saving directorysizes cache", "path", directorySizesPath, "isTruncate", b.noFilterApply)
			// When all selections are made, the cache file is rewritten.
			// (To delete old entries that are no longer needed.)
			if err := dirCache.Save(trashDir.Dir, b.noFilterApply); err != nil { // if
				slog.Warn("failed to save directorysizes cache", "path", directorySizesPath, "error", err)
			}
		}

		b.TrashDirs = append(b.TrashDirs, trashDir.Dir)
		if len(files) > 0 {
			// TODO: perf: run only when necessary
			sortFiles(files, b.sortBy, b.ascend)
			b.FilesByTrashDir[trashDir.Dir] = files
		}
		b.Files = append(b.Files, files...)
	}

	if len(b.Files) == 0 {
		return fmt.Errorf("%w: trashed files", ErrNotFound)
	}

	sortFiles(b.Files, b.sortBy, b.ascend)

	// truncate to last n items
	if b.limitLast > 0 {
		if len(b.Files) > b.limitLast {
			n := len(b.Files)
			b.Files = b.Files[n-b.limitLast : n]
		}
	}

	return nil
}

func (b *Box) getFiles(dirents []fs.DirEntry, fileEntries map[string]bool, trashDir xdg.TrashDir, dirCache xdg.DirCache, dirCacheUpdated *bool) []File {
	var files []File
	for _, ent := range dirents {
		if ent.Type().IsRegular() && strings.HasSuffix(ent.Name(), ".trashinfo") {
			trashInfoPath := filepath.Join(trashDir.InfoDir(), ent.Name())

			if strings.HasPrefix(ent.Name(), "._") {
				// exclude mac resource fork
				slog.Debug("skipped mac resource fork of .trashinfo", "path", trashInfoPath)
				continue
			}

			f, err := os.Open(trashInfoPath)
			if err != nil {
				slog.Warn("failed to open .trashinfo, skipped", "path", trashInfoPath, "error", err)
				continue
			}

			info, err := xdg.NewInfo(f)

			// It is better to close each time from a performance standpoint.
			f.Close()

			if err != nil {
				slog.Warn("failed to parse .trashinfo, skipped", "path", trashInfoPath, "error", err)
				continue
			}

			if !strings.HasPrefix(info.Path, string(os.PathSeparator)) {
				// If it was a relative path, convert it to an absolute path
				info.Path = filepath.Join(trashDir.Root, info.Path)
			}

			trashFileName := strings.TrimSuffix(ent.Name(), ".trashinfo")

			file := File{
				Name:          filepath.Base(info.Path),
				OriginalPath:  info.Path,
				TrashPath:     filepath.Join(trashDir.FilesDir(), trashFileName),
				TrashInfoPath: trashInfoPath,
				DeletedAt:     info.DeletionDate,
				IsDir:         fileEntries[trashFileName],
			}

			// If the corresponding trashed file does not exist, it is assumed to be invalid metadata and skipped
			if _, ok := fileEntries[trashFileName]; !ok {
				slog.Debug("file in the meta information does not exist, skipped", "trashInfoPath", file.TrashInfoPath, "trashPath", file.TrashPath)
				b.OrphanMeta = append(b.OrphanMeta, file)
				continue
			}

			// filter by directory
			if b.directory != "" {
				subpath, _ := posix.CheckSubPath(b.directory, file.OriginalPath)
				if !subpath {
					continue
				}
			}

			// filter by original path
			if len(b.queries) > 0 {
				switch b.queryModeBy {
				case ModeByFull:
					if !slices.Contains(b.queries, file.OriginalPath) {
						continue
					}
				case ModeByLiteral:
					var match bool
					for _, q := range b.queries {
						if strings.Contains(file.OriginalPath, q) {
							match = true
							break
						}
					}
					if !match {
						continue
					}
				case ModeByRegex:
					var match bool
					for _, reg := range b.queriesReg {
						if reg.MatchString(file.OriginalPath) {
							match = true
							break
						}
					}
					if !match {
						continue
					}
				case ModeByGlob:
					var match bool
					for _, glob := range b.queriesGlob {
						if glob.Match(file.OriginalPath) {
							match = true
							break
						}
					}
					if !match {
						continue
					}
				}
			}

			// filter by deletedAt
			if b.day > 0 {
				if b.newer {
					if b.dayPoint.After(info.DeletionDate) {
						continue
					}
				} else {
					if b.dayPoint.Before(info.DeletionDate) {
						continue
					}
				}
			}

			// calculate file or directory size
			if b.GetSize {
				fi, err := os.Lstat(file.TrashPath)
				if err != nil {
					slog.Warn("cannot lstat(2) to the trashed file for getting size", "trashPath", file.TrashPath, "error", err)
					goto BREAK_GET_SIZE
				}

				file.Mode = fi.Mode()
				file.IsDir = fi.IsDir()
				if !fi.IsDir() {
					// if regular file

					// Files can be retrieved by stat.
					s := fi.Size()
					file.Size = &s
					goto BREAK_GET_SIZE
				}

				// For directory, refer to cache and recursively calculate size if cache misses

				// Check the update time of the trashinfo file to see if the cache has become stale
				fi, err = os.Stat(file.TrashInfoPath)
				if err != nil {
					// Since the file has already been loaded, it is unlikely to reach this point
					slog.Warn("cannot stat(2) to the trashinfo file for calculating directory size", "trashInfoPath", file.TrashInfoPath, "error", err)
					goto BREAK_GET_SIZE
				}

				// if directory, get size recursively while referring to cache
				var size int64
				// check cache entry
				if item, ok := dirCache[trashFileName]; ok && item.Item.Mtime.Unix() == fi.ModTime().Unix() {
					// cache hit and cache is not stale
					size = item.Item.Size
					item.Seen = true
				} else {
					if item == nil {
						slog.Debug("calculating directory size", "reason", "CACHE_NOT_HIT", "trashPath", file.TrashPath)
					} else {
						slog.Debug("calculating directory size", "reason", "CACHE_STALE", "trashPath", file.TrashPath)
					}
					*dirCacheUpdated = true

					// calculate directory size
					s, err := posix.DirSizeFallback(file.TrashPath)
					if err != nil {
						// Even if rename(2) succeeds, the file inside may not be readable depending on the permissions.
						slog.Warn("cannot calculate directory size", "trashPath", file.TrashPath, "error", err)

						// Delete from cache because size could not be retrieved
						// noop when there is no cache
						delete(dirCache, trashFileName)

						goto BREAK_GET_SIZE
					}
					size = s

					// update cache
					if item == nil {
						// cache not hit

						// add cache entry
						dirCache[trashFileName] = &struct {
							Item xdg.DirCacheItem
							Seen bool
						}{
							Item: xdg.DirCacheItem{
								Size:    size,
								Mtime:   fi.ModTime(),
								DirName: trashFileName,
							},
							Seen: true,
						}
					} else {
						// cache hit but stale

						// update new size and mtime
						item.Item.Size = size
						item.Item.Mtime = fi.ModTime()
						item.Seen = true
					}
				}

				// succeed to get folder size
				file.Size = &size
			}

		BREAK_GET_SIZE:
			// filter by size
			if b.sizeHuman != "" { // See sizeHuman to allow filtering even with 0
				// If the size is not obtained, it is nil then skipped.
				if file.Size == nil {
					continue
				}

				if b.sizeLarger {
					if uint64(*file.Size) < b.size {
						continue
					}
				} else {
					if uint64(*file.Size) > b.size {
						continue
					}
				}
			}

			b.hitByPath[file.OriginalPath]++

			files = append(files, file)
		}
	}

	return files
}

// TODO: refactor
func sortFiles(files []File, sortBy SortByType, ascend bool) {
	switch sortBy {
	case SortByDeletedAt: // default
		sort.Slice(files, func(i, j int) bool {
			if !ascend {
				i, j = j, i
			}
			return files[i].DeletedAt.Before(files[j].DeletedAt)
		})
	case SortBySize:
		sort.Slice(files, func(i, j int) bool {
			if !ascend {
				i, j = j, i
			}

			// If size is not available, treat as less than 0
			si, sj := files[i].Size, files[j].Size

			var minus int64 = -1
			if si == nil {
				si = &minus
			}
			if sj == nil {
				sj = &minus
			}

			return *si < *sj
		})
	case SortByName:
		sort.Slice(files, func(i, j int) bool {
			if !ascend {
				i, j = j, i
			}
			return files[i].OriginalPath < files[j].OriginalPath
		})
	}
}

func (b *Box) HitByPath(originalPath string) int {
	return b.hitByPath[originalPath]
}

type File struct {
	Name          string    // .vimrc
	OriginalPath  string    // ~/.vimrc (Info.Path)
	TrashPath     string    // ~/.local/share/Trash/files/.vimrc
	TrashInfoPath string    // ~/.local/share/Trash/info/.vimrc.trashinfo
	DeletedAt     time.Time // 2023-01-01T00:00:00 (Info.DeletionDate)
	IsDir         bool
	// optionals below
	Size *int64 // nil if could not get, It may not be able to be taken due to permission violation, etc.
	Mode fs.FileMode
}

type Group struct {
	Dir         string
	IsDirCommon bool      // Whether Dir is the same for all files
	DeletedAt   time.Time // pick one from Files
	Files       []File
}

func (f *File) OriginalPathFormat(tilde bool, color bool) string {
	p := f.OriginalPath
	if tilde {
		p = posix.AbsPathToTilde(p)
	}
	if color {
		return f.pathColor(p)
	} else {
		return p
	}
}

func (f *File) TrashPathColor() string {
	return f.pathColor(f.TrashPath)
}

func (f *File) SizeHuman() string {
	if f.Size == nil {
		return "-"
	} else {
		return humanize.Bytes(uint64(*f.Size))
	}
}

func (f *File) pathColor(s string) string {
	var color lipgloss.Color

	if f.IsDir {
		color = lipgloss.Color("12") // blue
	} else if f.Mode != 0 {
		switch {
		case f.Mode&0o111 > 0: // may be binary (x flag being set)
			color = lipgloss.Color("9") // red
		}
	}

	return lipgloss.NewStyle().Foreground(color).Render(s)
}

func (b *Box) ToGroups() []Group {
	files := b.Files

	// group by deletedAt
	filesByDeletedAt := make(map[time.Time][]File)
	for _, file := range files {
		filesByDeletedAt[file.DeletedAt] = append(filesByDeletedAt[file.DeletedAt], file)
	}

	hasMultiDirs := func(files []File) bool {
		var dirs []string
		unique := make(map[string]bool)
		for _, file := range files {
			dir := filepath.Dir(file.OriginalPath)
			if !unique[dir] {
				unique[dir] = true
				dirs = append(dirs, dir)
			}
		}
		return len(dirs) > 1
	}

	var groups []Group
	for deletedAt, files := range filesByDeletedAt {
		dir := filepath.Dir(files[0].OriginalPath)
		isDirCommon := true

		if hasMultiDirs(files) {
			dir = "(multiple directories)"
			isDirCommon = false
		}
		groups = append(groups, Group{
			Dir:         dir,
			DeletedAt:   deletedAt,
			Files:       files,
			IsDirCommon: isDirCommon,
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].DeletedAt.After(groups[j].DeletedAt)
	})

	return groups
}

func (f *File) Delete() error {
	slog.Debug("removing .trashinfo", "trashInfoPath", f.TrashInfoPath)
	return os.Remove(f.TrashInfoPath)
}
