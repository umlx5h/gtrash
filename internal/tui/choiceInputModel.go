package tui

import (
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type choiceInputModel struct {
	textInput    textinput.Model
	keys         map[string]string
	defaultValue *string
	confirmed    bool
}

func newChoiceInputModel(prompt string, choices []string, defaultValue *string) choiceInputModel {
	textInput := textinput.New()
	textInput.Prompt = prompt
	textInput.Placeholder = strings.Join(choices, "/")
	if defaultValue != nil {
		textInput.Placeholder += ", default " + *defaultValue
	}

	keys := make(map[string]string)
	for _, choice := range choices {
		keys[choice[0:1]] = choice
	}
	textInput.Validate = func(s string) error {
		if s == "" && defaultValue != nil {
			return nil
		}
		if _, ok := keys[s]; ok {
			return nil
		}
		return errors.New("unknown")
	}
	textInput.Focus()
	return choiceInputModel{
		textInput:    textInput,
		keys:         keys,
		defaultValue: defaultValue,
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
			return m, tea.Quit
		case tea.KeyEnter:
			value := m.textInput.Value()
			if value == "" && m.defaultValue != nil {
				// Enter pressed
				m.textInput.SetValue(*m.defaultValue)
				m.confirmed = true
				m.textInput.Blur()
				return m, tea.Quit
			} else if value, ok := m.keys[value]; ok {
				m.textInput.SetValue(value)
				m.confirmed = true
				m.textInput.Blur()
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	if _, ok := m.keys[m.textInput.Value()]; ok {
		m.confirmed = true
		m.textInput.Blur()
		return m, tea.Quit
	}
	return m, cmd
}

func (m choiceInputModel) Value() string {
	value := m.textInput.Value()
	if value == "" && m.defaultValue != nil {
		return *m.defaultValue
	}
	return m.keys[value]
}

func (m choiceInputModel) View() string {
	return m.textInput.View() + "\n"
}
