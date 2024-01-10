package posix

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/umlx5h/go-runewidth"
)

func IsBinary(content io.ReadSeeker, fileSize int64) (bool, error) {
	headSize := min(fileSize, 1024)
	head := make([]byte, headSize)
	if _, err := content.Read(head); err != nil {
		return false, err
	}
	if _, err := content.Seek(0, io.SeekStart); err != nil {
		return false, err
	}

	// ref: https://github.com/file/file/blob/5e33fd6ee7766d40382a084c8e7554c2d43c0b7e/src/encoding.c#L183-L260
	for _, b := range head {
		if b < 7 || b == 11 || (13 < b && b < 27) || (27 < b && b < 0x20) || b == 0x7f {
			return true, nil
		}
	}

	return false, nil
}

func FileHead(path string, width int, maxLines int) string {
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "(error: not found)"
		} else {
			return "(error: could not stat)"
		}
	}
	content := func(isDir bool, lines []string) string {
		if len(lines) == 0 {
			if isDir {
				return "(empty directory)\n"
			} else {
				return "(empty file)\n"
			}
		}
		var content string
		var i int
		for _, line := range lines {
			i++
			content += fmt.Sprintf("  %s\n", line)
		}
		if isDir {
			return "(directory)" + "\n" + content
		} else {
			return "(text)" + "\n" + content
		}
	}

	var lines []string
	var isDir bool
	switch {
	case fi.Mode().Type() == fs.ModeSymlink:
		return "(symbolic link)"
	case fi.IsDir():
		isDir = true
		dirs, _ := os.ReadDir(path)
		for i, dir := range dirs {
			if i == maxLines {
				break
			}
			dinfo, err := dir.Info()
			if err != nil {
				return "(error: open directory)"
			}
			name := runewidth.Truncate(dir.Name(), width-15, "…")
			if dir.IsDir() {
				// folder is blue color
				name = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render(name)
			}
			l := fmt.Sprintf("%s  %s", dinfo.Mode().Perm().String(), name)
			lines = append(lines, l)
		}
	case fi.Mode().IsRegular():
		f, err := os.Open(path)
		if err != nil {
			return "(error: open file)"
		}
		defer f.Close()

		if binary, err := IsBinary(f, fi.Size()); err != nil {
			return "(error: read file)"
		} else if binary {
			return "(binary file)"
		}

		// if file is text, read maxLines lines
		s := bufio.NewScanner(f)
		var n int
		for s.Scan() {
			if n == maxLines {
				break
			}
			t := s.Text()
			// truncate to screen width
			lines = append(lines, runewidth.Truncate(t, width-3, "…"))
			n++
		}
	default:
		return "(unknown file type)"
	}
	return content(isDir, lines)
}

func FileType(st fs.FileInfo) string {
	if st.IsDir() {
		return "directory"
	} else if st.Mode().IsRegular() {
		if st.Size() == 0 {
			return "regular empty file"
		} else {
			return "regular file"
		}
	}

	switch st.Mode().Type() {
	case fs.ModeSymlink:
		return "symbolic link"
	case fs.ModeNamedPipe:
		return "fifo"
	case fs.ModeSocket:
		return "socket"
	}

	return "unknown type file"
}
