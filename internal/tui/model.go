package tui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/N3M1K/xrp/internal/scanner"
	"github.com/N3M1K/xrp/internal/socket"
	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	baseStyle     = lipgloss.NewStyle().Padding(1, 2)
	titleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("36")).Bold(true).MarginBottom(1)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Bold(true)
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	tunnelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

type tickMsg time.Time

type model struct {
	processes []scanner.Process
	cursor    int
	tld       string
	err       error
	msg       string
}

func InitialModel(tld string) *model {
	return &model{
		processes: []scanner.Process{},
		tld:       tld,
	}
}

func Start(tld string) error {
	p := tea.NewProgram(InitialModel(tld))
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(fetchProcessesCmd, tickCmd())
}

func fetchProcessesCmd() tea.Msg {
	resp, err := socket.Send(socket.Request{Cmd: "list"})
	if err != nil || !resp.Success {
		return fmt.Errorf("Daemon offline")
	}
	var procs []scanner.Process
	json.Unmarshal(resp.Data, &procs)
	return procs
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case []scanner.Process:
		m.processes = msg
		m.err = nil
		if m.cursor >= len(m.processes) {
			m.cursor = len(m.processes) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
		}

	case error:
		m.err = msg

	case tickMsg:
		return m, tea.Batch(fetchProcessesCmd, tickCmd())

	case tea.KeyMsg:
		m.msg = "" // clear ephemeral message
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.processes)-1 {
				m.cursor++
			}

		case "enter", "o":
			if len(m.processes) > 0 {
				p := m.processes[m.cursor]
				url := fmt.Sprintf("http://localhost:%d", p.Port)
				if p.ProjectName != "" {
					url = fmt.Sprintf("https://%s.%s", p.ProjectName, m.tld)
				}
				socket.Send(socket.Request{
					Cmd:  "open",
					Args: map[string]string{"url": url},
				})
				m.msg = "Opening browser..."
			}

		case "s":
			if len(m.processes) > 0 {
				p := m.processes[m.cursor]
				if p.ProjectName != "" {
					m.msg = "Starting cloudflared tunnel..."
					go func(port int, proj string) {
						socket.Send(socket.Request{
							Cmd:  "share",
							Args: map[string]string{"port": strconv.Itoa(port), "project": proj},
						})
					}(p.Port, p.ProjectName)
					return m, fetchProcessesCmd // immediately fetch
				} else {
					m.msg = "Cannot share process without a project name."
				}
			}

		case "u":
			if len(m.processes) > 0 {
				p := m.processes[m.cursor]
				if p.ProjectName != "" && p.TunnelURL != "" {
					m.msg = "Stopping cloudflared tunnel..."
					go func(proj string) {
						socket.Send(socket.Request{
							Cmd:  "unshare",
							Args: map[string]string{"project": proj},
						})
					}(p.ProjectName)
					return m, fetchProcessesCmd // immediately fetch
				}
			}
			
		case "c":
			if len(m.processes) > 0 {
				p := m.processes[m.cursor]
				if p.TunnelURL != "" {
					clipboard.WriteAll(p.TunnelURL)
					m.msg = "Copied tunnel URL to clipboard!"
				}
			}
		}
	}

	return m, nil
}

func (m *model) View() string {
	s := titleStyle.Render("🚀 XRP Dashboard")
	s += "\n"

	if m.err != nil {
		s += errorStyle.Render(fmt.Sprintf("Error: %v (Is daemon running? Run 'xrp start')", m.err))
		s += "\n\n"
	} else if len(m.processes) == 0 {
		s += subtleStyle.Render("No active local development servers found.")
		s += "\n\n"
	} else {
		// Table Header
		s += lipgloss.JoinHorizontal(lipgloss.Left,
			fmt.Sprintf("%-6s ", "PORT"),
			fmt.Sprintf("| %-20s ", "PROJECT"),
			fmt.Sprintf("| %-15s ", "APP"),
			fmt.Sprintf("| %-40s", "STATUS / TUNNEL"),
		) + "\n"
		s += subtleStyle.Render("-----------------------------------------------------------------------------------------") + "\n"

		for i, p := range m.processes {
			cursor := " "
			rowStyle := lipgloss.NewStyle()
			if m.cursor == i {
				cursor = ">"
				rowStyle = selectedStyle
			}

			proj := p.ProjectName
			if proj == "" {
				proj = "-"
			}
			app := p.KnownApp
			if app == "" {
				app = "-"
			}

			status := "Local only"
			if p.TunnelURL != "" {
				status = tunnelStyle.Render("🌐 " + p.TunnelURL)
			}

			row := fmt.Sprintf("%s %-6d | %-20s | %-15s | %s", cursor, p.Port, proj, app, status)
			s += rowStyle.Render(row) + "\n"
		}
		s += "\n"
	}

	if m.msg != "" {
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(m.msg) + "\n\n"
	} else {
		s += "\n"
	}

	s += subtleStyle.Render("↑/↓: navigate • enter/o: open browser • s: share • u: unshare • c: copy tunnel URL • q: quit")
	return baseStyle.Render(s)
}
