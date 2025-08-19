package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func Wait(ctx context.Context, duration time.Duration) chan struct{} {
	stopChan := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(duration):
			stopChan <- struct{}{}
		}
	}()
	return stopChan
}

type stopMsg struct{}

type ModelSpinner struct {
	spinner  spinner.Model
	message  string
	nextView func() tea.Model
	stopChan chan struct{}
}

func NewModelSpinner(msg string, stopChan chan struct{}, nextView func() tea.Model) ModelSpinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return ModelSpinner{
		spinner:  s,
		message:  msg,
		nextView: nextView,
		stopChan: stopChan,
	}
}

func (m ModelSpinner) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			return stopMsg(<-m.stopChan)
		},
	)
}

func (m ModelSpinner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case stopMsg:
		return m.nextView(), nil
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m ModelSpinner) View() string {
	str := fmt.Sprintf("\n\n   %s %s...press q to quit\n\n", m.spinner.View(), m.message)
	return str
}
