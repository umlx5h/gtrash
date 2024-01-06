package tui

import (
	"fmt"
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
)

var _ tea.Model = singleRestoreModel{}

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type singleRestoreModel struct {
	width       int
	height      int
	fixedWidth  int
	tableHeight int

	wrapStyle lipgloss.Style

	table table.Model
	input textinput.Model // filter

	groups []trash.Group // data source

	keymap keymap
	help   help.Model

	confirmed bool // true when confirmed by pressing Enter
	selected  int  // groups index,

	hit, total, hitWidth int
}

func makeGroupRow(idx int, g trash.Group) table.Row {
	return []string{
		strconv.Itoa(idx + 1),
		humanize.Time(g.DeletedAt),
		strconv.Itoa(len(g.Files)),
		posix.AbsPathToTilde(g.Dir),
	}
}

func newSingleRestoreModel(groups []trash.Group) singleRestoreModel {
	width, height := getTermSize()

	var (
		noWidth    = len(strconv.Itoa(len(groups)))
		dateWidth  int
		filesWidth int
	)
	if noWidth <= 1 {
		noWidth = 2
	}

	rows := make([]table.Row, len(groups))
	for i, g := range groups {
		r := makeGroupRow(i, g)

		// Only ASCII characters are used, so it should match the character length
		w := len(r[1])
		if w > dateWidth {
			dateWidth = w
		}
		w = len(r[2])
		if w > filesWidth {
			filesWidth = w
		}

		rows[i] = r
	}
	filesWidth = max(filesWidth, len("Files"))

	paddingWidth := 5 * 2 // (columns + 1) * 2

	fixedWidth := noWidth + dateWidth + filesWidth + paddingWidth

	pathWidth := width - fixedWidth

	columns := []table.Column{
		{Title: "No", Width: noWidth},
		{Title: "DeletedAt", Width: dateWidth},
		{Title: "Files", Width: filesWidth},
		{Title: "RestoreDir", Width: pathWidth},
	}

	tableHeight := int(float64(height)*0.55) - paddingHeight

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(tableHeight),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("240")).
		Bold(true)
	t.SetStyles(s)

	i := textinput.New()
	i.PromptStyle = greyStyle
	i.Cursor.Style = inputCursorStyle

	h := baseHelp
	km := baseKeymap

	m := singleRestoreModel{
		width:       width,
		height:      height,
		fixedWidth:  fixedWidth,
		tableHeight: tableHeight,

		wrapStyle: lipgloss.NewStyle().Width(width).Height(height).MaxWidth(width).MaxHeight(height),

		table:  t,
		input:  i,
		groups: groups,

		keymap: km,
		help:   h,

		total:    len(rows),
		hit:      len(rows),
		hitWidth: len(strconv.Itoa(len(rows))),
	}

	m.updateInputPrompt()

	return m
}

func (m singleRestoreModel) Init() tea.Cmd {
	return nil
}

func (m singleRestoreModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// when focused to table
		if m.table.Focused() {
			switch {
			case key.Matches(msg, m.keymap.quit):
				return m, tea.Quit
			case key.Matches(msg, m.keymap.filter):
				m.table.Blur()
				m.input.Focus()
				return m, nil
			case key.Matches(msg, m.keymap.clear):
				if m.input.Value() != "" {
					m.input.Reset()
					m.filterApply()
				}
				return m, nil
			case key.Matches(msg, m.keymap.runRestore):
				selected := m.table.SelectedRow()
				if selected == nil {
					return m, nil
				}

				idx, err := strconv.Atoi(selected[0])
				if err != nil {
					panic(err)
				}

				m.confirmed = true
				m.selected = idx - 1

				return m, tea.Quit
			}

			m.table, cmd = m.table.Update(msg)
			return m, cmd
		} else if m.input.Focused() {
			// when focused to filter textinput
			switch msg.String() {
			case "enter", "esc":
				m.input.Blur()
				m.table.Focus()
			case "ctrl+c":
				m.input.Blur()
				m.table.Focus()
				return m, nil
			}
			m.input, cmd = m.input.Update(msg)
			// Reflecting filter to the table
			m.filterApply()

			return m, cmd
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateScreenSize()
	}

	return m, nil
}

func (m *singleRestoreModel) updateScreenSize() {
	m.wrapStyle = m.wrapStyle.Width(m.width).Height(m.height).MaxWidth(m.width).MaxHeight(m.height)
	m.table.SetColWidthLast(m.width - m.fixedWidth)

	newHeight := int(float64(m.height)*0.55 - paddingHeight)
	if newHeight < 1 {
		newHeight = 0
	}
	m.table.SetHeight(newHeight)
	m.tableHeight = newHeight
}

func (m *singleRestoreModel) updateInputPrompt() {
	m.input.Prompt = fmt.Sprintf("Trash Group %*d/%d > ", m.hitWidth, m.hit, m.total)

}

func (m *singleRestoreModel) updateHit() {
	m.total = len(m.groups)
	m.hit = len(m.table.Rows())

	m.updateInputPrompt()
}

func (m *singleRestoreModel) filterApply() {
	// Move the cursor to the top as the record changes
	m.table.GotoTop()

	var rows []table.Row
	for i, g := range m.groups {
		for _, f := range g.Files {
			// search by original path
			if m.input.Value() == "" || findMatch(f.OriginalPath, m.input.Value()) {

				rows = append(rows, makeGroupRow(i, g))
				break
			}
		}
	}

	m.table.SetRows(rows)
	m.updateHit()
}

func (m singleRestoreModel) View() string {
	var body strings.Builder

	body.WriteString(" " + m.input.View() + "\n")
	body.WriteString(baseStyle.Render(m.table.View()) + "\n")

	help := m.help.ShortHelpView([]key.Binding{
		m.keymap.quit,
		m.keymap.runRestore,
		m.keymap.filter,
		m.keymap.clear,
		m.keymap.pageup,
		m.keymap.pagedown,
	})
	body.WriteString(help + "\n")

	body.WriteString(m.viewMetadata())

	// Use wrapStyle to prevent buggy table display when moving the screen width repeatedly.
	return m.wrapStyle.Render(body.String())
}

func (m singleRestoreModel) viewMetadata() string {
	var body strings.Builder

	row := m.table.SelectedRow()

	if row == nil {
		return ""
	}

	selected, _ := strconv.Atoi(row[0])
	g := m.groups[selected-1]

	body.WriteString(greyStyle.Render("DeletedAt:      ") + "\t" + fmt.Sprintf("%s (%s)", g.DeletedAt.Format(time.DateTime), row[1]) + "\n")
	body.WriteString(greyStyle.Render("RestoreDir:     ") + "\t" + g.Dir + "\n")
	body.WriteString(greyStyle.Render("Number of Files:") + "\t" + row[2] + "\n")
	body.WriteString(greyStyle.Render("Files:") + "\n")

	// TODO: make scrollable
	for i, f := range g.Files {
		// TODO: calculation correct?
		if i > m.height-m.tableHeight-11 {
			break
		}
		body.WriteString("  - " + f.OriginalPathFormat(false, true) + "\n")
	}

	return body.String()
}
