package tui

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/umlx5h/gtrash/internal/trash"
)

func FilesSelect(files []trash.File) ([]trash.File, error) {
	m := newMultiRestoreModel(files)
	result, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	if r, ok := result.(multiRestoreModel); ok {
		if r.confirmed {
			return r.restoreFiles, nil
		}
	}

	return nil, errors.New("no selected")
}

func GroupSelect(groups []trash.Group) (trash.Group, error) {
	m := newSingleRestoreModel(groups)
	result, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	if r, ok := result.(singleRestoreModel); ok {
		if r.confirmed {
			return groups[r.selected], nil
		}
	}

	return trash.Group{}, errors.New("no selected")
}

func BoolPrompt(prompt string) bool {
	m := newBoolInputModel(prompt)

	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return false
	}

	if m, ok := result.(boolInputModel); ok {
		return m.Confirmed() && m.Value()
	}

	return false
}

func ChoicePrompt(prompt string, choices []string) (string, error) {
	model := newChoiceInputModel(prompt, choices)
	result, err := tea.NewProgram(model).Run()
	if err != nil {
		return "", err
	}

	if m, ok := result.(choiceInputModel); ok {
		if !m.Confirmed() || m.Value() == "quit" { // hard code quit
			return "", errors.New("canceled")
		}

		return m.Value(), err
	}
	return "", errors.New("unexpected error in ChoicePrompt")
}
