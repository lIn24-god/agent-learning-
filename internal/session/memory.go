// 长期记忆可以按用户或项目分层存储。这里我们先实现一个用户级的简单键值存储（记忆事实）

package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type LongTermMemory struct {
	basePath string
	filePath string
	mem      map[string]string
}

// NewLongTermMemory 创建长期记忆，userID 可用来区分用户（这里传任意字符串，比如 "default"）
func NewLongTermMemory(basePath, userID string) (*LongTermMemory, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}
	filePath := filepath.Join(basePath, userID+".json")
	mem := &LongTermMemory{
		basePath: basePath,
		filePath: filePath,
		mem:      make(map[string]string),
	}
	// 加载已有记忆
	data, err := os.ReadFile(filePath)
	if err == nil {
		json.Unmarshal(data, &mem.mem)
	}
	return mem, nil
}

// Remember 记住一个键值对
func (m *LongTermMemory) Remember(key, value string) error {
	m.mem[key] = value
	return m.save()
}

// Recall 回忆某个键对应的值
func (m *LongTermMemory) Recall(key string) (string, bool) {
	v, ok := m.mem[key]
	return v, ok
}

// GetContext 将所有记忆拼成一段文本，供系统提示使用
func (m *LongTermMemory) GetContext() string {
	if len(m.mem) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("【长期记忆】\n")
	for k, v := range m.mem {
		b.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
	}
	return b.String()
}

func (m *LongTermMemory) save() error {
	data, err := json.MarshalIndent(m.mem, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0644)
}
