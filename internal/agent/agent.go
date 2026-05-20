package agent

import (
	"context"
	"fmt"
	"lagent/internal/config"
	"lagent/internal/session"
	"strings"
)

type Agent struct {
	config      *config.Config
	activeModel string
	clients     map[string]ModelClient
	sessMgr     *session.Manager
	longMem     *session.LongTermMemory
}

// NewAgent 创建 Agent，同时初始化会话管理和长期记忆
func NewAgent(cfg *config.Config) (*Agent, error) {
	// 初始化模型客户端（代码与之前相同，省略）
	clients := make(map[string]ModelClient)
	for name, modelCfg := range cfg.Models {
		switch modelCfg.Type {
		case "openai_compatible":
			clients[name] = NewDeepSeekClient(
				modelCfg.APIKey,
				modelCfg.BaseURL,
				modelCfg.ModelName,
				modelCfg.Temperature,
			)
		case "ollama":
			oc, err := NewOllamaClient(modelCfg.BaseURL, modelCfg.ModelName, modelCfg.Temperature)
			if err != nil {
				return nil, fmt.Errorf("初始化 Ollama 失败 (%s): %w", name, err)
			}
			clients[name] = oc
		default:
			return nil, fmt.Errorf("未知模型类型: %s", modelCfg.Type)
		}
	}
	if _, ok := clients[cfg.DefaultModel]; !ok {
		return nil, fmt.Errorf("默认模型 %s 未定义", cfg.DefaultModel)
	}

	// 初始化会话管理器（存储路径从配置读取）
	sessMgr, err := session.NewManager(cfg.MemoryPath + "/sessions")
	if err != nil {
		return nil, fmt.Errorf("初始化会话管理器失败: %w", err)
	}
	// 长期记忆（用户级，这里使用固定用户 "default"，可根据需要扩展）
	longMem, err := session.NewLongTermMemory(cfg.MemoryPath+"/longmem", "default")
	if err != nil {
		return nil, fmt.Errorf("初始化长期记忆失败: %w", err)
	}

	return &Agent{
		config:      cfg,
		activeModel: cfg.DefaultModel,
		clients:     clients,
		sessMgr:     sessMgr,
		longMem:     longMem,
	}, nil
}

// SetModel 切换模型
func (a *Agent) SetModel(modelName string) error {
	if _, ok := a.clients[modelName]; !ok {
		return fmt.Errorf("模型 %s 不存在", modelName)
	}
	a.activeModel = modelName
	return nil
}

// GetSessionManager 暴露给 TUI 使用
func (a *Agent) GetSessionManager() *session.Manager {
	return a.sessMgr
}

// NewSession 创建新会话（CLI 命令 /new）
func (a *Agent) NewSession() {
	a.sessMgr.NewSession()
}

// SwitchSession 切换会话（/resume）
func (a *Agent) SwitchSession(id string) error {
	return a.sessMgr.SwitchToSession(id)
}

// StreamGenerate 增强版：自动拼接历史 + 长期记忆 + 保存对话
func (a *Agent) StreamGenerate(ctx context.Context, prompt string, callback func(chunk string)) error {
	// 1. 获取历史上下文（最近 10 条消息）
	history := a.sessMgr.GetHistoryPrompt(10)
	// 2. 获取长期记忆文本
	longMemText := a.longMem.GetContext()
	// 3. 构建完整 prompt
	var fullPrompt strings.Builder
	if longMemText != "" {
		fullPrompt.WriteString(longMemText)
		fullPrompt.WriteString("\n")
	}
	if history != "" {
		fullPrompt.WriteString(history)
	}
	fullPrompt.WriteString(fmt.Sprintf("用户: %s\n助手:", prompt))

	// 先保存用户消息（注意：在流式完成后再保存助手回复）
	if err := a.sessMgr.AddMessage("user", prompt); err != nil {
		return fmt.Errorf("保存用户消息失败: %w", err)
	}

	// 4. 流式调用模型
	var fullResponse strings.Builder
	client := a.clients[a.activeModel]
	err := client.StreamGenerate(ctx, fullPrompt.String(), func(chunk string) {
		fullResponse.WriteString(chunk)
		callback(chunk)
	})
	if err != nil {
		// 如果出错，可以从会话中移除刚刚添加的用户消息（可选）
		return err
	}

	// 5. 保存助手回复
	if err := a.sessMgr.AddMessage("assistant", fullResponse.String()); err != nil {
		return fmt.Errorf("保存助手消息失败: %w", err)
	}

	return nil
}
