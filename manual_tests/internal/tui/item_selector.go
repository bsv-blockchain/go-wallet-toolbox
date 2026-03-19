package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
)

type ItemSelector[T ~string] struct {
	cursor   int
	items    []T
	title    string
	onSelect func(T) (tea.Model, tea.Cmd)
	onBack   func() (tea.Model, tea.Cmd)
	showBack bool
}

// NewItemSelector creates a basic ItemSelector without back navigation
func NewItemSelector[T ~string](items []T, title string, onSelect func(T) (tea.Model, tea.Cmd)) ItemSelector[T] {
	return ItemSelector[T]{
		items:    items,
		title:    title,
		onSelect: onSelect,
		showBack: false,
	}
}

// NewItemSelectorWithBack creates an ItemSelector with back navigation
func NewItemSelectorWithBack[T ~string](items []T, title string, onSelect func(T) (tea.Model, tea.Cmd), onBack func() (tea.Model, tea.Cmd)) ItemSelector[T] {
	return ItemSelector[T]{
		items:    items,
		title:    title,
		onSelect: onSelect,
		onBack:   onBack,
		showBack: true,
	}
}

func (m ItemSelector[T]) Init() tea.Cmd {
	return nil
}

func (m ItemSelector[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.Type { //nolint:exhaustive // only specific keys handled, others ignored
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
	case tea.KeyEnter:
		return m.handleEnterKey()
	case tea.KeyDown, tea.KeyTab, tea.KeyCtrlN:
		m.moveDown()
		return m, nil
	case tea.KeyUp, tea.KeyShiftTab, tea.KeyCtrlP:
		m.moveUp()
		return m, nil
	}

	switch keyMsg.String() {
	case "q":
		return m, tea.Quit
	case "j":
		m.moveDown()
	case "k":
		m.moveUp()
	}

	return m, nil
}

func (m ItemSelector[T]) handleEnterKey() (tea.Model, tea.Cmd) {
	if m.isBackOptionSelected() {
		return m.handleBackSelection()
	}
	return m.handleItemSelection()
}

func (m ItemSelector[T]) isBackOptionSelected() bool {
	return m.showBack && m.cursor == len(m.items)
}

func (m ItemSelector[T]) handleBackSelection() (tea.Model, tea.Cmd) {
	if m.onBack != nil {
		newModel, newCmd := m.onBack()
		if newModel != nil {
			return newModel, newCmd
		}
	}
	return m, nil
}

func (m ItemSelector[T]) handleItemSelection() (tea.Model, tea.Cmd) {
	newModel, newCmd := m.onSelect(m.items[m.cursor])
	if newModel != nil {
		return newModel, newCmd
	}
	return m, nil
}

func (m *ItemSelector[T]) moveDown() {
	maxCursor := m.getMaxCursorPosition()
	m.cursor++
	if m.cursor > maxCursor {
		m.cursor = 0
	}
}

func (m *ItemSelector[T]) moveUp() {
	maxCursor := m.getMaxCursorPosition()
	m.cursor--
	if m.cursor < 0 {
		m.cursor = maxCursor
	}
}

func (m ItemSelector[T]) getMaxCursorPosition() int {
	maxCursor := len(m.items) - 1
	if m.showBack {
		maxCursor = len(m.items)
	}
	return maxCursor
}

func (m ItemSelector[T]) View() string {
	s := strings.Builder{}
	fmt.Fprintf(&s, "%s\n\n", m.title)

	for i, item := range m.items {
		if m.cursor == i {
			s.WriteString("(•) ")
		} else {
			s.WriteString("( ) ")
		}
		s.WriteString(string(item))
		s.WriteString("\n")
	}

	if m.showBack {
		if m.cursor == len(m.items) {
			s.WriteString("(•) ")
		} else {
			s.WriteString("( ) ")
		}
		s.WriteString(fixtures.ButtonBack)
		s.WriteString("\n")
	}

	s.WriteString("\n(press q to quit)\n")

	return s.String()
}
