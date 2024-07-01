package tui

import (
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type boolInputModel struct {
	textInput textinput.Model
	confirmed bool
}

func yesno(s string) (bool, string, error) {
	if s == "" {
		return false, "", errors.New("empty")
	}
	switch strings.ToLower(s[0:1]) {
	case "y":
		return true, "Yes", nil
	case "n":
		return false, "No", nil
	}
	return false, "", errors.New("unknown")
}

func newBoolInputModel(prompt string) boolInputModel {
	textInput := textinput.New()
	textInput.Prompt = prompt
	textInput.Placeholder = "(Yes/No)"
	textInput.Validate = func(value string) error {
		_, _, err := yesno(value)
		return err
	}
	textInput.Focus()
	return boolInputModel{
		textInput: textInput,
	}
}

func (m boolInputModel) Confirmed() bool {
	return m.confirmed
}

func (m boolInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m boolInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.textInput.Blur()
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	if _, value, err := yesno(m.textInput.Value()); err == nil {
		m.textInput.Blur()
		m.textInput.SetValue(value)
		m.confirmed = true
		return m, tea.Quit
	}
	return m, cmd
}

func (m boolInputModel) Value() bool {
	valueStr := m.textInput.Value()
	v, _, _ := yesno(valueStr)
	return v
}

func (m boolInputModel) View() string {
	return m.textInput.View() + "\n"
}
