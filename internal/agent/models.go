package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sashabaranov/go-openai"
)

// ModelClient 是所有模型客户端需要实现的接口
type ModelClient interface {
	StreamGenerate(ctx context.Context, prompt string, callback func(chunk string)) error
}

// DeepSeekClient 实现 ModelClient
type DeepSeekClient struct {
	client      *openai.Client
	modelName   string
	temperature float32
}

// NewDeepSeekClient 创建 DeepSeek 客户端
func NewDeepSeekClient(apiKey, baseURL, modelName string, temperature float32) *DeepSeekClient {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL
	return &DeepSeekClient{
		client:      openai.NewClientWithConfig(config),
		modelName:   modelName,
		temperature: temperature,
	}
}

func (c *DeepSeekClient) StreamGenerate(ctx context.Context, prompt string, callback func(chunk string)) error {
	req := openai.ChatCompletionRequest{
		Model: c.modelName,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Stream:      true,
		Temperature: c.temperature,
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return fmt.Errorf("deepseek 流式请求失败: %w", err)
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("接收 deepseek 流式响应错误: %w", err)
		}
		if len(response.Choices) > 0 {
			chunk := response.Choices[0].Delta.Content
			if chunk != "" {
				callback(chunk)
			}
		}
	}
	return nil
}

// OllamaClient 直接通过 HTTP 实现，避免第三方包版本问题
type OllamaClient struct {
	baseURL     string
	modelName   string
	temperature float32
	httpClient  *http.Client
}

// NewOllamaClient 创建 Ollama 客户端
func NewOllamaClient(baseURL, modelName string, temperature float32) (*OllamaClient, error) {
	return &OllamaClient{
		baseURL:     baseURL,
		modelName:   modelName,
		temperature: temperature,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// StreamGenerate 实现流式生成
func (c *OllamaClient) StreamGenerate(ctx context.Context, prompt string, callback func(chunk string)) error {
	// 构建请求体
	reqBody := map[string]interface{}{
		"model":  c.modelName,
		"prompt": prompt,
		"stream": true,
		"options": map[string]interface{}{
			"temperature": c.temperature,
		},
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("请求 ollama 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama 返回错误状态码: %d", resp.StatusCode)
	}

	// 逐行读取 JSON 流
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var chunk struct {
			Response string `json:"response"`
			Done     bool   `json:"done"`
		}
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue // 忽略无效行
		}
		if chunk.Response != "" {
			callback(chunk.Response)
		}
		if chunk.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取流失败: %w", err)
	}
	return nil
}
