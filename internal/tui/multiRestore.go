package tui

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/umlx5h/gtrash/internal/posix"
	"github.com/umlx5h/gtrash/internal/trash"
	"github.com/umlx5h/gtrash/internal/tui/table"
	"golang.org/x/term"
)

const (
	paddingHeight = 6
	shortWidth    = 90
)

var notFocusBorderStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

var focusBorderStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("70"))

var greyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("246"))

var inputCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("70"))

var baseHelp = help.New()

// 'Konsole Terminal' will collapse the table display, but if the width is not shortened, the layout will collapse even further, so it should be handled individually.
var isKonsole bool

var (
	focusRowStyle    = table.DefaultStyles()
	notFocusRowStyle = table.DefaultStyles()
)

func init() {
	focusRowStyle.Header = focusRowStyle.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("70")).
		BorderBottom(true).
		Bold(false)
	focusRowStyle.Selected = focusRowStyle.Selected.
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("240")).
		Bold(true)

	notFocusRowStyle.Header = notFocusRowStyle.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	notFocusRowStyle.Selected = notFocusRowStyle.Selected.
		Foreground(lipgloss.Color("246")).
		Background(lipgloss.Color("240")).
		Bold(false)

	// help text color lighter
	baseHelp.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#909090",
		// Dark:  "#626262",
		Dark: "246",
	})
	baseHelp.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#B2B2B2",
		// Dark:  "#4A4A4A",
		Dark: "242",
	})
	baseHelp.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#DDDADA",
		// Dark:  "#3C3C3C",
		Dark: "239",
	})
	baseHelp.Styles.Ellipsis = baseHelp.Styles.ShortSeparator.Copy()
	baseHelp.Styles.FullKey = baseHelp.Styles.ShortKey.Copy()
	baseHelp.Styles.FullDesc = baseHelp.Styles.ShortDesc.Copy()
	baseHelp.Styles.FullSeparator = baseHelp.Styles.ShortSeparator.Copy()

	if _, ok := os.LookupEnv("KONSOLE_VERSION"); ok {
		isKonsole = true
	}
}

var baseKeymap = keymap{
	quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q/CTRL-C", "quit"),
	),
	runRestore: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("Enter", "restore"),
	),
	filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	clear: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("ESC", "clear filter"),
	),
	pageup: key.NewBinding(
		key.WithKeys("u", "pgup"),
		key.WithHelp("u/PageUp", "page up"),
	),
	pagedown: key.NewBinding(
		key.WithKeys("d", "pgdn"),
		key.WithHelp("d/PageDown", "page down"),
	),
	top: key.NewBinding(
		key.WithKeys("g", "home"),
		key.WithHelp("g/Home", "go to top"),
	),
	bottom: key.NewBinding(
		key.WithKeys("G", "end"),
		key.WithHelp("G/End", "go to bottom"),
	),
}

type keymap struct {
	help, quit, focus, moveRight, moveLeft, runRestore, filter                                      key.Binding
	move, moveRightALL, moveLeftALL, filterCWD, clear, pageup, pagedown, top, bottom, togglePreview key.Binding
}

type filterTable struct {
	title string

	t     table.Model
	input textinput.Model

	hit, total, hitWidth int // updated when filtering
}

func (t *filterTable) getSelectedIdx() int {
	idx, err := strconv.Atoi(t.t.SelectedRow()[0])
	if err != nil {
		panic(err)
	}

	return idx - 1
}

func (t *filterTable) updateInputPrompt(cwd bool) {
	// TODO: Set hit to "-" when no filter is applied
	// (to distinguish between unfiltered and all hits)
	if cwd {
		t.input.Prompt = fmt.Sprintf("%s (cwd) %*d/%d > ", t.title, t.hitWidth, t.hit, t.total)
	} else {
		t.input.Prompt = fmt.Sprintf("%s %*d/%d > ", t.title, t.hitWidth, t.hit, t.total)
	}
}

func (m *multiRestoreModel) updateHit() {
	m.trashTable.total = len(m.files) - len(m.selected)
	m.trashTable.hit = len(m.trashTable.t.Rows())

	m.trashTable.updateInputPrompt(m.filterCWD)

	m.restoreTable.total = len(m.selected)
	m.restoreTable.hit = len(m.restoreTable.t.Rows())

	m.restoreTable.updateInputPrompt(false)
}

// Convert from rows to an array of indices
func (t *filterTable) getIndices() []int {
	rows := t.t.Rows()
	if len(rows) == 0 {
		return nil
	}

	indices := make([]int, len(rows))
	for n, r := range rows {
		idx, err := strconv.Atoi(r[0])
		if err != nil {
			panic(err)
		}

		indices[n] = idx - 1
	}
	return indices
}

func (t filterTable) View(focus bool) string {
	var body strings.Builder

	body.WriteString(" " + t.input.View() + "\n")
	if focus {
		body.WriteString(focusBorderStyle.Render(t.t.View()))
	} else {
		body.WriteString(notFocusBorderStyle.Render(t.t.View()))
	}

	return body.String()
}

var _ tea.Model = multiRestoreModel{}

type multiRestoreModel struct {
	width       int
	height      int
	fixedWidth  int
	tableHeight int

	wrapStyle lipgloss.Style

	keymap keymap
	help   help.Model

	trashTable   *filterTable // left table
	restoreTable *filterTable // right table

	files []trash.File // table source

	selected    map[int]struct{} // selected the indices of files
	rightFocus  bool             // focus to restoreTable
	showHelp    bool
	showPreview bool

	filterCWD bool
	filesCWD  map[int]struct{} // Specify the indices of files when filtered by cwd

	confirmed    bool         // true when confirmed by pressing Enter
	restoreFiles []trash.File // return value
}

func (m *multiRestoreModel) getFocusTable() (focus *filterTable, notFocus *filterTable) {
	if !m.rightFocus {
		return m.trashTable, m.restoreTable
	} else {
		return m.restoreTable, m.trashTable
	}
}

func (m *multiRestoreModel) getRestoreFiles() []trash.File {
	if len(m.selected) == 0 {
		return nil
	}
	files := make([]trash.File, len(m.selected))

	indices := make([]int, len(m.selected))
	var i int
	for idx := range m.selected {
		indices[i] = idx
		i++
	}
	sort.Slice(indices, func(i, j int) bool {
		return indices[i] < indices[j]
	})

	for i, idx := range indices {
		files[i] = m.files[idx]
	}

	return files
}

func makeFileRow(idx int, f trash.File) table.Row {
	return []string{
		strconv.Itoa(idx + 1),
		humanize.Time(f.DeletedAt),
		// Prevent color display problems with table records
		strings.TrimSuffix(f.OriginalPathFormat(true, true), "\033[0m"),
	}
}

func getTermSize() (width int, height int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		panic(err)
	}

	return w, h
}

func makeFilterTables(files []trash.File) (left, right filterTable, fixedWidth, tableHeight int) {
	width, height := getTermSize()

	var (
		dateWidth int
		noWidth   = len(strconv.Itoa(len(files)))
	)
	if noWidth <= 1 {
		noWidth = 2
	}
	if isKonsole {
		noWidth += 1
	}

	rows := make([]table.Row, len(files))
	for i, f := range files {
		r := makeFileRow(i, f)

		// Only ASCII characters are used, so it should match the character length
		w := len(r[1])
		if w > dateWidth {
			dateWidth = w
		}
		rows[i] = r
	}

	paddingWidth := 4 * 2 // (columns + 1) * 2

	fixedWidth = noWidth + dateWidth + paddingWidth

	// make table shorter
	if isKonsole {
		fixedWidth += 4
	}
	pathWidth := (width / 2) - fixedWidth

	// Must be separate instances to prevent data race
	getColumn := func() []table.Column {
		columns := []table.Column{
			{Title: "No", Width: noWidth},
			{Title: "DeletedAt", Width: dateWidth},
			{Title: "Path", Width: pathWidth},
		}
		return columns
	}

	tableHeight = int(float64(height)*0.55) - paddingHeight

	leftInput := textinput.New()
	leftInput.PromptStyle = greyStyle
	leftInput.Cursor.Style = inputCursorStyle

	left = filterTable{
		title:    "Trash",
		total:    len(rows),
		hit:      len(rows),
		hitWidth: len(strconv.Itoa(len(rows))),

		t: table.New(
			table.WithColumns(getColumn()),
			table.WithHeight(tableHeight),
			table.WithRows(rows),
			table.WithFocused(true),
			table.WithStyles(focusRowStyle),
			table.WithShortColumn(1, 2),
		),
		input: leftInput,
	}
	left.updateInputPrompt(false)

	rightInput := textinput.New()
	rightInput.PromptStyle = greyStyle
	rightInput.Cursor.Style = inputCursorStyle

	right = filterTable{
		title: "Restore",

		hit:      0,
		total:    0,
		hitWidth: left.hitWidth,

		t: table.New(
			table.WithColumns(getColumn()),
			table.WithHeight(tableHeight),
			table.WithStyles(notFocusRowStyle),
			table.WithShortColumn(1, 2),
		),
		input: rightInput,
	}
	right.t.SetShortMode(true)
	right.updateInputPrompt(false)

	return left, right, fixedWidth, tableHeight
}

func newMultiRestoreModel(files []trash.File) multiRestoreModel {
	trashTable, restoreTable, fixedWidth, tableHeight := makeFilterTables(files)
	width, height := getTermSize()

	h := baseHelp

	km := baseKeymap
	km.help = key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	)
	km.focus = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("TAB", "focus"),
	)
	km.moveRight = key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "move right"),
	)
	km.moveLeft = key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "move left"),
	)
	km.moveRightALL = key.NewBinding(
		key.WithKeys("L"),
		key.WithHelp("L", "move right all"),
	)
	km.moveLeftALL = key.NewBinding(
		key.WithKeys("H"),
		key.WithHelp("H", "move left all"),
	)
	km.move = key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("Space", "move other side"),
	)
	km.filterCWD = key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "filter by cwd"),
	)
	km.togglePreview = key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "toggle preview"),
	)

	m := multiRestoreModel{
		trashTable:   &trashTable,
		restoreTable: &restoreTable,

		width:       width,
		height:      height,
		fixedWidth:  fixedWidth,
		tableHeight: tableHeight,

		wrapStyle: lipgloss.NewStyle().Width(width).Height(height).MaxWidth(width).MaxHeight(height),

		showPreview: true,

		files:    files,
		help:     h,
		selected: make(map[int]struct{}),
		keymap:   km,
	}

	return m
}

func (m *multiRestoreModel) updateScreenSize() {
	m.wrapStyle = m.wrapStyle.Width(m.width).Height(m.height).MaxWidth(m.width).MaxHeight(m.height)
	if m.width < shortWidth {
		m.trashTable.t.SetShortMode(true)
	} else {
		m.trashTable.t.SetShortMode(false)
	}
	// symmetry
	// newWidth := m.width/2 - m.fixedWidth
	// m.trashTable.t.SetColWidthLast(newWidth)
	// m.restoreTable.t.SetColWidthLast(newWidth)

	// Make the table on the left a little larger.
	m.trashTable.t.SetColWidthLast(int(float64(m.width)*0.6) - m.fixedWidth)
	m.restoreTable.t.SetColWidthLast(int(float64(m.width)*0.4) - m.fixedWidth)

	newHeight := int(float64(m.height)*0.55 - paddingHeight)
	if newHeight < 1 {
		newHeight = 0
	}
	m.trashTable.t.SetHeight(newHeight)
	m.restoreTable.t.SetHeight(newHeight)

	m.tableHeight = newHeight
}

func (m multiRestoreModel) Init() tea.Cmd {
	return nil
}

func (m multiRestoreModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	ft, nft := m.getFocusTable()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// when focused to table
		if ft.t.Focused() {
			switch {
			case key.Matches(msg, m.keymap.focus):
				ft.t.Blur()
				ft.t.SetStyles(notFocusRowStyle)

				nft.t.Focus()
				nft.t.SetStyles(focusRowStyle)
				m.rightFocus = !m.rightFocus

			case key.Matches(msg, m.keymap.moveRight):
				if !m.rightFocus {
					m.moveRow()
				}
			case key.Matches(msg, m.keymap.moveLeft):
				if m.rightFocus {
					m.moveRow()
				}
			case key.Matches(msg, m.keymap.move):
				m.moveRow()
			case key.Matches(msg, m.keymap.moveRightALL):
				if !m.rightFocus {
					m.moveRowALL()
				}
			case key.Matches(msg, m.keymap.moveLeftALL):
				if m.rightFocus {
					m.moveRowALL()
				}
			case key.Matches(msg, m.keymap.quit):
				return m, tea.Quit
			case key.Matches(msg, m.keymap.filter):
				ft.t.Blur()
				ft.input.Focus()
				return m, nil
			case key.Matches(msg, m.keymap.clear):
				if ft.input.Value() != "" {
					ft.input.Reset()
					m.filterApply()
				}
				return m, nil
			case key.Matches(msg, m.keymap.help):
				m.showHelp = !m.showHelp
				return m, nil
			case key.Matches(msg, m.keymap.togglePreview):
				m.showPreview = !m.showPreview
				return m, nil
			case key.Matches(msg, m.keymap.filterCWD):
				// only used in left table
				if m.rightFocus {
					return m, nil
				}
				m.filterCWD = !m.filterCWD

				if m.filterCWD && m.filesCWD == nil {
					// Filter by cwd the first time it is called and cache the results
					cwd, err := os.Getwd()
					if err != nil {
						return m, nil
					}

					filesCWD := make(map[int]struct{})
					for i, f := range m.files {
						subpath, _ := posix.CheckSubPath(cwd, f.OriginalPath)
						if !subpath {
							continue
						}
						filesCWD[i] = struct{}{}
					}
					m.filesCWD = filesCWD
				}

				m.trashTable.input.Reset()
				if m.filterCWD {
					m.trashTable.t.SetColumnNameLast("Path (cwd)")
				} else {
					m.trashTable.t.SetColumnNameLast("Path")
				}

				m.filterApply()

				return m, nil
			case key.Matches(msg, m.keymap.runRestore):
				files := m.getRestoreFiles()
				// If the file to be restored is not selected, nothing is done.
				if len(files) == 0 {
					return m, nil
				}

				m.confirmed = true
				m.restoreFiles = files

				return m, tea.Quit
			}

			ft.t, cmd = ft.t.Update(msg)
			return m, cmd

		} else if ft.input.Focused() {
			// when focused to filter textinput
			switch msg.String() {
			case "enter", "esc":
				ft.input.Blur()
				ft.t.Focus()
			case "ctrl+c":
				ft.input.Blur()
				ft.t.Focus()
				return m, nil
			}
			ft.input, cmd = ft.input.Update(msg)

			// Reflecting filter to the table
			m.filterApply()

			return m, cmd
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateScreenSize()
	}

	// ft.t, cmd = ft.t.Update(msg)
	// return m, cmd
	return m, nil
}

func deleteRow(rows []table.Row, cursor int) []table.Row {
	return rows[:cursor+copy(rows[cursor:], rows[cursor+1:])]
}

func addRows(rows []table.Row, adds []table.Row) []table.Row {
	if len(rows) == 0 {
		return adds
	}

	// TODO: perf
	rows = append(rows, adds...)
	sort.Slice(rows, func(i, j int) bool {
		i, _ = strconv.Atoi(rows[i][0])
		j, _ = strconv.Atoi(rows[j][0])

		return i < j
	})

	return rows
}

func addRow(rows []table.Row, add table.Row) []table.Row {
	if len(rows) == 0 {
		return []table.Row{add}
	}
	// TODO: perf
	rows = append(rows, add)
	sort.Slice(rows, func(i, j int) bool {
		i, _ = strconv.Atoi(rows[i][0])
		j, _ = strconv.Atoi(rows[j][0])

		return i < j
	})

	return rows
}

func (m *multiRestoreModel) moveRow() {
	from := m.trashTable
	to := m.restoreTable

	if m.rightFocus {
		from = m.restoreTable
		to = m.trashTable
	}

	if from.t.SelectedRow() == nil {
		return
	}

	// apply to selected
	idx := from.getSelectedIdx()
	if !m.rightFocus {
		m.selected[idx] = struct{}{}
	} else {
		delete(m.selected, idx)
	}

	// delete row from focus table
	rows := from.t.Rows()
	selectedRow := from.t.SelectedRow()
	cursor := from.t.Cursor()
	if len(rows) >= 2 && len(rows) == cursor+1 {
		// When the last line is selected, shift the focus up one line
		from.t.SetCursor(cursor - 1)
	}

	rows = deleteRow(rows, cursor)
	from.t.SetRows(rows)

	// add to other side table if filter matches
	if to.input.Value() == "" || findMatch(selectedRow[len(selectedRow)-1], to.input.Value()) {
		rows = to.t.Rows()
		rows = addRow(rows, selectedRow)

		to.t.SetRows(rows)
	}
	m.updateHit()
}

func (m *multiRestoreModel) moveRowALL() {
	from := m.trashTable
	to := m.restoreTable

	if m.rightFocus {
		from = m.restoreTable
		to = m.trashTable
	}

	if len(from.t.Rows()) == 0 {
		return
	}

	// apply to selected
	for _, idx := range from.getIndices() {
		if !m.rightFocus {
			m.selected[idx] = struct{}{}
		} else {
			delete(m.selected, idx)
		}
	}

	// delete all rows from focus table
	rows := from.t.Rows()
	from.t.SetCursor(0)
	from.t.SetRows(nil)

	// add to other side table if filter matches
	if to.input.Value() == "" { // if filter not used, append all
		rows = addRows(to.t.Rows(), rows)
		to.t.SetRows(rows)
	} else {
		filterRows := make([]table.Row, 0)

		for _, r := range rows {
			if findMatch(r[len(r)-1], to.input.Value()) {
				filterRows = append(filterRows, r)
			}
		}

		rows = addRows(to.t.Rows(), filterRows)
		to.t.SetRows(rows)
	}

	m.updateHit()
}

func (m *multiRestoreModel) filterApply() {
	ft, _ := m.getFocusTable()

	// Move the cursor to the top as the record changes
	ft.t.GotoTop()

	var rows []table.Row
	for i, f := range m.files {
		// Exclude already selected rows from filtering
		if !m.rightFocus {
			if _, ok := m.selected[i]; ok {
				continue
			}

			// Apply cwd filtering
			if m.filterCWD {
				if _, ok := m.filesCWD[i]; !ok {
					continue
				}
			}

		} else {
			if _, ok := m.selected[i]; !ok {
				continue
			}
		}

		if ft.input.Value() == "" || findMatch(f.OriginalPath, ft.input.Value()) {
			rows = append(rows, makeFileRow(i, f))
		}
	}

	ft.t.SetRows(rows)
	m.updateHit()
}

func findMatch(text, pattern string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
}

func (m multiRestoreModel) View() string {
	var body strings.Builder

	body.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, m.trashTable.View(!m.rightFocus), m.restoreTable.View(m.rightFocus)))

	if m.showHelp {
		help := m.help.FullHelpView([][]key.Binding{
			{
				m.keymap.moveRight,
				m.keymap.moveLeft,
				m.keymap.move,
				m.keymap.moveRightALL,
				m.keymap.moveLeftALL,
			},
			{
				m.keymap.focus,
				m.keymap.filter,
				m.keymap.filterCWD,
				m.keymap.clear,
				m.keymap.togglePreview,
			},
			{
				m.keymap.pageup,
				m.keymap.runRestore,
				m.keymap.pagedown,
				m.keymap.top,
				m.keymap.bottom,
			},
			{
				m.keymap.quit,
				m.keymap.help,
			},
		})
		body.WriteString("\n" + help)
	} else {
		help := m.help.ShortHelpView([]key.Binding{
			m.keymap.help,
			m.keymap.quit,
			m.keymap.focus,
			m.keymap.moveRight,
			m.keymap.moveLeft,
			m.keymap.runRestore,
			m.keymap.filter,
		})
		body.WriteString("\n" + help)

		body.WriteString("\n" + m.viewMetadata())
	}

	// Use wrapStyle to prevent buggy table display when moving the screen width repeatedly.
	return m.wrapStyle.Render(body.String())
}

func (m multiRestoreModel) viewMetadata() string {
	ft, _ := m.getFocusTable()

	var body strings.Builder

	if ft.t.SelectedRow() == nil {
		return ""
	}

	f := m.files[ft.getSelectedIdx()]

	body.WriteString(greyStyle.Render("FileName:        ") + f.Name + "\n")
	body.WriteString(greyStyle.Render("OriginalPath:    ") + f.OriginalPathFormat(false, true) + "\n")
	body.WriteString(greyStyle.Render("TrashPath:       ") + f.TrashPathColor() + "\n")
	body.WriteString(greyStyle.Render("DeletedAt:       ") + fmt.Sprintf("%s (%s)", f.DeletedAt.Format(time.DateTime), ft.t.SelectedRow()[1]) + "\n")

	if m.showPreview {
		body.WriteString(greyStyle.Render("Preview:         ") + posix.FileHead(f.TrashPath, m.width, m.height-m.tableHeight-paddingHeight-6))
	}

	return body.String()
}
