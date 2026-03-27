package tui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/N3M1K/xrp/internal/config"
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
	inputStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

type tickMsg time.Time

// editingTLDMsg is returned after a TLD save succeeds or fails
type editingTLDMsg struct {
	err     error
	project string
	tld     string
}

type model struct {
	processes  []scanner.Process
	cursor     int
	tld        string
	cfg        *config.Config
	err        error
	msg        string
	// TLD editing state
	editingTLD bool
	tldInput   string
}

func InitialModel(tld string, cfg *config.Config) *model {
	return &model{
		processes: []scanner.Process{},
		tld:       tld,
		cfg:       cfg,
	}
}

func Start(tld string, cfg *config.Config) error {
	p := tea.NewProgram(InitialModel(tld, cfg))
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

func saveTLDCmd(project, tld string) tea.Cmd {
	return func() tea.Msg {
		err := config.SetProjectTLD(project, tld)
		return editingTLDMsg{err: err, project: project, tld: tld}
	}
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

	case editingTLDMsg:
		m.editingTLD = false
		if msg.err != nil {
			m.msg = fmt.Sprintf("❌ Failed to save TLD: %v", msg.err)
		} else {
			// Update in-memory config so TLD column & URL update immediately
			if m.cfg != nil {
				if m.cfg.ProjectTLDs == nil {
					m.cfg.ProjectTLDs = make(map[string]string)
				}
				m.cfg.ProjectTLDs[msg.project] = msg.tld
			}
			m.msg = fmt.Sprintf("✅ TLD saved! %s.%s — daemon will apply on next poll.", msg.project, msg.tld)
		}
		return m, fetchProcessesCmd

	case tickMsg:
		return m, tea.Batch(fetchProcessesCmd, tickCmd())

	case tea.KeyMsg:
		// --- TLD editing mode ---
		if m.editingTLD {
			switch msg.String() {
			case "esc":
				m.editingTLD = false
				m.tldInput = ""
				m.msg = "Cancelled."
			case "enter":
				if len(m.processes) == 0 {
					m.editingTLD = false
					return m, nil
				}
				project := m.processes[m.cursor].ProjectName
				tld := strings.TrimPrefix(strings.TrimSpace(m.tldInput), ".")
				if project == "" {
					m.editingTLD = false
					m.msg = "❌ No project name for this service."
					return m, nil
				}
				return m, saveTLDCmd(project, tld)
			case "backspace":
				if len(m.tldInput) > 0 {
					m.tldInput = m.tldInput[:len(m.tldInput)-1]
				}
			default:
				// Accept printable characters
				if len(msg.String()) == 1 {
					m.tldInput += msg.String()
				}
			}
			return m, nil
		}

		// --- Normal mode ---
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
					effectiveTLD := m.tld
					if custom := m.getProjectTLD(p.ProjectName); custom != "" {
						effectiveTLD = custom
					}
					url = fmt.Sprintf("https://%s.%s", p.ProjectName, effectiveTLD)
				}
				socket.Send(socket.Request{
					Cmd:  "open",
					Args: map[string]string{"url": url},
				})
				m.msg = fmt.Sprintf("Opening %s...", url)
			}

		case "t":
			if len(m.processes) > 0 {
				p := m.processes[m.cursor]
				if p.ProjectName == "" {
					m.msg = "❌ Cannot set TLD: no project name for this service."
				} else {
					m.editingTLD = true
					// Pre-fill with the project's current custom TLD, or fall back to global TLD
					if current := m.getProjectTLD(p.ProjectName); current != "" {
						m.tldInput = current
					} else {
						m.tldInput = m.tld
					}
					m.msg = ""
				}
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
					return m, fetchProcessesCmd
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
					return m, fetchProcessesCmd
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
			fmt.Sprintf("| %-15s ", "TLD"),
			fmt.Sprintf("| %-40s", "STATUS / TUNNEL"),
		) + "\n"
		s += subtleStyle.Render("-------------------------------------------------------------------------------------------------------------") + "\n"

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

			// Resolve TLD for this specific project
			effectiveTLD := m.tld
			if p.ProjectName != "" {
				if custom := m.getProjectTLD(p.ProjectName); custom != "" {
					effectiveTLD = custom
				}
			}

			status := "Local only"
			if p.TunnelURL != "" {
				status = tunnelStyle.Render("🌐 " + p.TunnelURL)
			}

			row := fmt.Sprintf("%s %-6d | %-20s | %-15s | %-15s | %s", cursor, p.Port, proj, app, effectiveTLD, status)
			s += rowStyle.Render(row) + "\n"
		}
		s += "\n"
	}

	// TLD editing overlay
	if m.editingTLD && len(m.processes) > 0 {
		proj := m.processes[m.cursor].ProjectName
		s += inputStyle.Render(fmt.Sprintf("Set TLD for '%s': .%s█", proj, m.tldInput)) + "\n"
		s += subtleStyle.Render("  enter: confirm • esc: cancel") + "\n\n"
	} else if m.msg != "" {
		if strings.HasPrefix(m.msg, "❌") {
			s += errorStyle.Render(m.msg) + "\n\n"
		} else {
			s += successStyle.Render(m.msg) + "\n\n"
		}
	} else {
		s += "\n"
	}

	s += subtleStyle.Render("↑/↓: navigate • enter/o: open browser • t: set TLD • s: share • u: unshare • c: copy URL • q: quit")
	return baseStyle.Render(s)
}

// getProjectTLD reads project-specific TLD from the config passed into the model.
func (m *model) getProjectTLD(project string) string {
	if m.cfg == nil || m.cfg.ProjectTLDs == nil {
		return ""
	}
	return strings.TrimPrefix(m.cfg.ProjectTLDs[project], ".")
}
