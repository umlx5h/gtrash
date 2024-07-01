package tui

import (
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type choiceInputModel struct {
	textInput textinput.Model
	keys      map[string]string
	confirmed bool
}

func newChoiceInputModel(prompt string, choices []string) choiceInputModel {
	textInput := textinput.New()
	textInput.Prompt = prompt

	for i := range choices {
		choices[i] = strings.ToUpper(choices[i][:1]) + choices[i][1:]
	}

	textInput.Placeholder = "(" + strings.Join(choices, "/") + ")"
	keys := make(map[string]string)
	for _, choice := range choices {
		keys[strings.ToLower(choice[0:1])] = choice
	}
	textInput.Validate = func(s string) error {
		if s == "" {
			return errors.New("empty")
		}
		if _, ok := keys[strings.ToLower(s[0:1])]; ok {
			return nil
		}
		return errors.New("unknown")
	}
	textInput.Focus()
	return choiceInputModel{
		textInput: textInput,
		keys:      keys,
	}
}

func (m choiceInputModel) Confirmed() bool {
	return m.confirmed
}

func (m choiceInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m choiceInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.textInput.Blur()
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	if value, ok := m.keys[strings.ToLower(m.textInput.Value())]; ok {
		m.textInput.Blur()
		m.textInput.SetValue(value)
		m.confirmed = true
		return m, tea.Quit
	}
	return m, cmd
}

func (m choiceInputModel) Value() string {
	value := m.textInput.Value()
	return strings.ToLower(value)
}

func (m choiceInputModel) View() string {
	return m.textInput.View() + "\n"
}
