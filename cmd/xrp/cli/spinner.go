package cli

import (
	"context"
	"fmt"

	"github.com/N3M1K/xrp/internal/deps"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type depsMsg struct {
	err error
}

type spinnerModel struct {
	spinner  spinner.Model
	quitting bool
	err      error
	ctx      context.Context
}

func initialSpinnerModel(ctx context.Context) spinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return spinnerModel{spinner: s, ctx: ctx}
}

func (m spinnerModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			_, err := deps.EnsureAll(m.ctx)
			return depsMsg{err: err}
		},
	)
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}
	case depsMsg:
		m.err = msg.err
		m.quitting = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m spinnerModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n❌ Failed to resolve dependencies: %v\n", m.err)
	}
	if m.quitting {
		return "✅ Dependencies securely provisioned!\n"
	}
	return fmt.Sprintf("\n   %s Orchestrating core binary requirements (caddy, mkcert, cloudflared)...\n\n", m.spinner.View())
}

func runSpinnerUI(ctx context.Context) error {
	p := tea.NewProgram(initialSpinnerModel(ctx))
	m, err := p.Run()
	if err != nil {
		return err
	}
	
	// Handle native Bubbletea context captures
	if model, ok := m.(spinnerModel); ok {
		return model.err
	}
	return nil
}
