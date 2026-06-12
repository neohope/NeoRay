package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"neoray/internal/agent"
	"neoray/internal/session"
)

// TUI 文本用户界面
type TUI struct {
	agent   *agent.Agent
	sessMgr *session.Manager
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

	p := tea.NewProgram(initialModel(t.agent, sess), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// Model Bubble Tea Model
type model struct {
	agent     *agent.Agent
	session   *session.Session
	textarea  textarea.Model
	viewport  viewport.Model
	messages  []messageItem
	err       error
	loading   bool
	streaming bool
	ready     bool
	width     int
}

type messageItem struct {
	Role    string
	Content string
}

type (
	responseMsg  string
	streamMsg    string
	streamEndMsg struct{}
	errMsg       error
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Bold(true)

	userMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#87D3F8")).
			Bold(true)

	assistantMsgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A78BFA")).
				Bold(true)

	borderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)
)

func initialModel(aiAgent *agent.Agent, sess *session.Session) model {
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Ctrl+C to exit)"
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.CharLimit = 0

	return model{
		agent:    aiAgent,
		session:  sess,
		textarea: ta,
		messages: []messageItem{},
		width:    80,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tea.EnterAltScreen)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			if !m.loading && !m.streaming && m.textarea.Value() != "" {
				input := m.textarea.Value()
				m.messages = append(m.messages, messageItem{Role: "user", Content: input})
				m.textarea.Reset()
				m.loading = true
				m.err = nil

				// 先添加占位消息
				m.messages = append(m.messages, messageItem{Role: "assistant", Content: ""})

				// 使用普通 Chat 调用
				return m, func() tea.Msg {
					ctx := context.Background()
					result, err := m.agent.Chat(ctx, m.session, input)
					if err != nil {
						return errMsg(err)
					}
					return responseMsg(result.Message.Content)
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		headerHeight := 1                                        // 简化
		footerHeight := 1                                        // 简化
		verticalMarginHeight := headerHeight + footerHeight + 10 // 给 textarea 留空间

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.textarea.SetWidth(msg.Width - 4)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
			m.textarea.SetWidth(msg.Width - 4)
		}

		// 更新 viewport 内容
		m.viewport.SetContent(m.messagesView())

	case streamMsg:
		m.loading = false
		m.streaming = true
		// 更新最后一条消息的内容
		if len(m.messages) > 0 {
			lastIdx := len(m.messages) - 1
			m.messages[lastIdx].Content += string(msg)
		}
		m.viewport.SetContent(m.messagesView())
		m.viewport.GotoBottom()

	case streamEndMsg:
		m.streaming = false

	case responseMsg:
		// 更新最后一条消息的内容
		if len(m.messages) > 0 {
			lastIdx := len(m.messages) - 1
			m.messages[lastIdx].Content = string(msg)
		}
		m.loading = false
		m.streaming = false
		m.viewport.SetContent(m.messagesView())
		m.viewport.GotoBottom()

	case errMsg:
		m.err = msg
		m.loading = false
		m.streaming = false
	}

	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	if m.ready {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) headerView() string {
	title := titleStyle.Render("NeoRay AI Assistant")
	var line string
	if m.ready {
		line = borderStyle.Render(strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title))))
	} else {
		line = borderStyle.Render(strings.Repeat("─", max(0, m.width-lipgloss.Width(title))))
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m model) footerView() string {
	var status string
	if m.loading {
		status = "⏳ Waiting for response..."
	} else if m.streaming {
		status = "🔄 Receiving response..."
	} else if m.err != nil {
		status = errorStyle.Render("❌ Error: " + m.err.Error())
	} else {
		status = "Ready"
	}

	var line string
	if m.ready {
		line = borderStyle.Render(strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(status))))
	} else {
		line = borderStyle.Render(strings.Repeat("─", max(0, m.width-lipgloss.Width(status))))
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, status, line)
}

func (m model) messagesView() string {
	var sb strings.Builder

	for _, msg := range m.messages {
		if msg.Role == "user" {
			sb.WriteString(userMsgStyle.Render("👤 You") + "\n")
			sb.WriteString("  " + strings.Join(strings.Split(msg.Content, "\n"), "\n  ") + "\n\n")
		} else {
			sb.WriteString(assistantMsgStyle.Render("🤖 Assistant") + "\n")
			content := msg.Content
			if content == "" && (m.loading || m.streaming) {
				content = "..."
			}
			sb.WriteString("  " + strings.Join(strings.Split(content, "\n"), "\n  ") + "\n\n")
		}
	}

	return sb.String()
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing NeoRay...\n\n  Please wait..."
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.headerView(),
		m.viewport.View(),
		m.footerView(),
		m.textarea.View(),
		"  (Enter to send, Ctrl+C to exit)",
	)
}
