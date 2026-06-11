package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbletea"

	"neoray/internal/agent"
	"neoray/internal/session"
)

// TUI 文本用户界面
type TUI struct {
	agent    *agent.Agent
	sessMgr  *session.Manager
}

// NewTUI 创建 TUI
func NewTUI(aiAgent *agent.Agent, sessMgr *session.Manager) *TUI {
	return &TUI{
		agent:   aiAgent,
		sessMgr: sessMgr,
	}
}

// Run 运行 TUI
func (t *TUI) Run() error {
	sess, err := t.sessMgr.CreateSession()
	if err != nil {
		return err
	}

	p := tea.NewProgram(initialModel(t.agent, sess))
	_, err = p.Run()
	return err
}

// Model Bubble Tea Model
type model struct {
	agent    *agent.Agent
	session  *session.Session
	textarea textarea.Model
	messages []messageItem
	err      error
	loading  bool
}

type messageItem struct {
	Role    string
	Content string
}

type (
	responseMsg string
	errMsg      error
)

func initialModel(aiAgent *agent.Agent, sess *session.Session) model {
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Ctrl+C to exit)"
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	return model{
		agent:    aiAgent,
		session:  sess,
		textarea: ta,
		messages: []messageItem{},
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			if !m.loading && m.textarea.Value() != "" {
				m.messages = append(m.messages, messageItem{Role: "user", Content: m.textarea.Value()})
				input := m.textarea.Value()
				m.textarea.Reset()
				m.loading = true
				return m, func() tea.Msg {
					ctx := context.Background()
					respMsg, err := m.agent.Chat(ctx, m.session, input)
					if err != nil {
						return errMsg(err)
					}
					return responseMsg(respMsg.Content)
				}
			}
		}

	case responseMsg:
		m.messages = append(m.messages, messageItem{Role: "assistant", Content: string(msg)})
		m.loading = false

	case errMsg:
		m.err = msg
		m.loading = false
	}

	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m model) View() string {
	var sb strings.Builder

	sb.WriteString("╔════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                    NeoRay AI Assistant                       ║\n")
	sb.WriteString("╚════════════════════════════════════════════════════════════╝\n\n")

	for _, msg := range m.messages {
		if msg.Role == "user" {
			sb.WriteString("👤 You:\n")
			sb.WriteString("   " + strings.Join(strings.Split(msg.Content, "\n"), "\n   ") + "\n\n")
		} else {
			sb.WriteString("🤖 Assistant:\n")
			sb.WriteString("   " + strings.Join(strings.Split(msg.Content, "\n"), "\n   ") + "\n\n")
		}
	}

	if m.loading {
		sb.WriteString("⏳ Waiting for response...\n\n")
	}

	if m.err != nil {
		sb.WriteString("❌ Error: " + m.err.Error() + "\n\n")
	}

	sb.WriteString("────────────────────────────────────────────────────────────────────\n")
	sb.WriteString(m.textarea.View())
	sb.WriteString("\n\n(Enter to send, Ctrl+C to exit)")

	return sb.String()
}
