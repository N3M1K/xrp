package cli

import (
	"context"
	"fmt"

	"github.com/N3M1K/halo-proxy/internal/deps"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type depsMsg struct {
	err error
	res deps.ResolvedDeps
}

type spinnerModel struct {
	spinner  spinner.Model
	quitting bool
	err      error
	tres     deps.ResolvedDeps
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
			res, err := deps.EnsureAll(m.ctx)
			return depsMsg{err: err, res: res}
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
		m.tres = msg.res
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
	return fmt.Sprintf("\n   %s Downloading and caching binary dependencies...\n\n", m.spinner.View())
}

func runSpinnerUI(ctx context.Context) (deps.ResolvedDeps, error) {
	// Bubble Tea needs a real TTY. Fall back to plain output so `halo start`
	// still works from scripts, CI, or a non-interactive shell.
	if !stdinIsTerminal() {
		fmt.Println("Downloading and caching binary dependencies...")
		res, err := deps.EnsureAll(ctx)
		if err != nil {
			return res, err
		}
		fmt.Println("✅ Dependencies provisioned.")
		return res, nil
	}

	p := tea.NewProgram(initialSpinnerModel(ctx))
	m, err := p.Run()
	if err != nil {
		return deps.ResolvedDeps{}, err
	}

	// Handle native Bubbletea context captures
	if model, ok := m.(spinnerModel); ok {
		return model.tres, model.err
	}
	return deps.ResolvedDeps{}, nil
}
