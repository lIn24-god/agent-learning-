package main

import (
	"context"
	"fmt"
	"lagent/internal/agent"
	"lagent/internal/config"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	agent    *agent.Agent
	viewport viewport.Model
	textarea textarea.Model
	messages []string
	renderer *glamour.TermRenderer
}

func (m *model) Init() tea.Cmd {
	return textarea.Blink
}

func (m *model) addMessage(msg string) {
	m.messages = append(m.messages, msg)
	m.updateViewport()
}

func (m *model) updateViewport() {
	content := strings.Join(m.messages, "\n\n")
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m *model) handleCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}
	switch parts[0] {
	case "/new":
		m.agent.NewSession()
		m.addMessage("🔄 已创建新会话")
	case "/list":
		sessions := m.agent.GetSessionManager().ListSessions()
		if len(sessions) == 0 {
			m.addMessage("📭 暂无历史会话")
		} else {
			m.addMessage("📋 历史会话列表:\n" + strings.Join(sessions, "\n"))
		}
	case "/resume":
		if len(parts) < 2 {
			m.addMessage("⚠️ 用法: /resume <会话ID>")
		} else {
			err := m.agent.SwitchSession(parts[1])
			if err != nil {
				m.addMessage("❌ " + err.Error())
			} else {
				m.addMessage("✅ 已切换到会话 " + parts[1])
			}
		}
	case "/exit":
		return tea.Quit
	default:
		m.addMessage("未知命令: " + parts[0])
	}
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			input := strings.TrimSpace(m.textarea.Value())
			m.textarea.Reset()
			if input == "" {
				return m, nil
			}
			if strings.HasPrefix(input, "/") {
				cmd := m.handleCommand(input)
				return m, cmd
			}
			// 正常消息：同步生成
			m.addMessage("你: " + input)
			m.addMessage("助手: ")
			m.updateViewport()

			var fullResp strings.Builder
			err := m.agent.StreamGenerate(context.Background(), input, func(chunk string) {
				fullResp.WriteString(chunk)
			})
			if err != nil {
				m.messages[len(m.messages)-1] = "助手: [错误] " + err.Error()
			} else {
				rendered, _ := m.renderer.Render(fullResp.String())
				m.messages[len(m.messages)-1] = "助手: " + rendered
			}
			m.updateViewport()
			return m, nil
		default:
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *model) View() string {
	return lipgloss.JoinVertical(
		lipgloss.Top,
		m.viewport.View(),
		m.textarea.View(),
	)
}

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	ag, err := agent.NewAgent(cfg)
	if err != nil {
		log.Fatal(err)
	}
	ta := textarea.New()
	ta.Placeholder = "输入消息... (/new, /list, /resume ID, /exit)"
	ta.Focus()
	ta.SetWidth(80)
	vp := viewport.New(80, 20)
	vp.SetContent("欢迎使用 Agent TUI！\n")
	renderer, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())
	p := tea.NewProgram(&model{
		agent:    ag,
		viewport: vp,
		textarea: ta,
		messages: []string{},
		renderer: renderer,
	})
	if _, err := p.Run(); err != nil {
		fmt.Printf("程序出错: %v", err)
	}
}
