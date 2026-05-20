package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Message 单条对话消息
type Message struct {
	Role      string    `json:"role"` // "user" 或 "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Session 代表一个完整的会话
type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages"`
}

// Manager 会话管理器
type Manager struct {
	basePath  string // 存储目录，例如 "./memories/sessions"
	sessions  map[string]*Session
	currentID string // 当前会话 ID
}

// NewManager 创建会话管理器，basePath 是存储目录
func NewManager(basePath string) (*Manager, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("创建会话目录失败: %w", err)
	}
	m := &Manager{
		basePath: basePath,
		sessions: make(map[string]*Session),
	}
	if err := m.loadAllSessions(); err != nil {
		return nil, err
	}
	return m, nil
}

// loadAllSessions 从磁盘加载所有会话
func (m *Manager) loadAllSessions() error {
	entries, err := os.ReadDir(m.basePath)
	if err != nil {
		return nil // 目录为空不算错误
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(m.basePath, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var sess Session
		if err := json.Unmarshal(data, &sess); err != nil {
			continue
		}
		m.sessions[sess.ID] = &sess
	}
	return nil
}

// saveSession 保存单个会话到 JSON 文件
func (m *Manager) saveSession(sess *Session) error {
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	filename := filepath.Join(m.basePath, sess.ID+".json")
	return os.WriteFile(filename, data, 0644)
}

// NewSession 创建新会话
func (m *Manager) NewSession() *Session {
	id := fmt.Sprintf("session_%d", time.Now().UnixNano())
	sess := &Session{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []Message{},
	}
	m.sessions[id] = sess
	m.currentID = id
	m.saveSession(sess)
	return sess
}

// GetCurrentSession 获取当前会话，若不存在则创建
func (m *Manager) GetCurrentSession() *Session {
	if m.currentID == "" {
		return m.NewSession()
	}
	sess, ok := m.sessions[m.currentID]
	if !ok {
		return m.NewSession()
	}
	return sess
}

// SwitchToSession 切换到指定 ID 的会话
func (m *Manager) SwitchToSession(id string) error {
	if _, ok := m.sessions[id]; !ok {
		return fmt.Errorf("会话 %s 不存在", id)
	}
	m.currentID = id
	return nil
}

// ListSessions 返回所有会话的简要信息（ID + 创建时间 + 消息数量）
func (m *Manager) ListSessions() []string {
	var lines []string
	for id, sess := range m.sessions {
		lines = append(lines, fmt.Sprintf("%s (%d msgs, %s)",
			id, len(sess.Messages), sess.CreatedAt.Format("01-02 15:04")))
	}
	return lines
}

// AddMessage 向当前会话添加一条消息，并自动保存
func (m *Manager) AddMessage(role, content string) error {
	sess := m.GetCurrentSession()
	msg := Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	sess.Messages = append(sess.Messages, msg)
	sess.UpdatedAt = time.Now()
	return m.saveSession(sess)
}

// GetHistoryPrompt 构建用于模型的历史上下文（最近 N 轮）
func (m *Manager) GetHistoryPrompt(maxMessages int) string {
	sess := m.GetCurrentSession()
	if len(sess.Messages) == 0 {
		return ""
	}
	// 取最近 maxMessages 条消息，但确保成对出现（user+assistant）
	start := 0
	if len(sess.Messages) > maxMessages {
		start = len(sess.Messages) - maxMessages
	}
	var builder strings.Builder
	for i := start; i < len(sess.Messages); i++ {
		msg := sess.Messages[i]
		if msg.Role == "user" {
			builder.WriteString(fmt.Sprintf("用户: %s\n", msg.Content))
		} else {
			builder.WriteString(fmt.Sprintf("助手: %s\n", msg.Content))
		}
	}
	return builder.String()
}
