package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type ItemSelector[T ~string] struct {
	cursor   int
	items    []T
	title    string
	onSelect func(T) (tea.Model, tea.Cmd)
}

func NewItemSelector[T ~string](items []T, title string, onSelect func(T) (tea.Model, tea.Cmd)) ItemSelector[T] {
	return ItemSelector[T]{
		items:    items,
		title:    title,
		onSelect: onSelect,
	}
}

func (m ItemSelector[T]) Init() tea.Cmd {
	return nil
}

func (m ItemSelector[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "enter":
			newModel, newCmd := m.onSelect(m.items[m.cursor])
			if newModel != nil {
				return newModel, newCmd
			}
		case "down", "j":
			m.cursor++
			if m.cursor >= len(m.items) {
				m.cursor = 0
			}

		case "up", "k":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.items) - 1
			}
		}
	}

	return m, nil
}

func (m ItemSelector[T]) View() string {
	s := strings.Builder{}
	s.WriteString(fmt.Sprintf("%s\n\n", m.title))

	for i, item := range m.items {
		if m.cursor == i {
			s.WriteString("(•) ")
		} else {
			s.WriteString("( ) ")
		}
		s.WriteString(string(item))
		s.WriteString("\n")
	}

	s.WriteString("\n(press q to quit)\n")

	return s.String()
}
