package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
)

type Client struct {
	client                 openai.Client
	model                  string
	tools                  []openai.ChatCompletionToolParam
	compressTokenThreshold int
}

// TokenUsage tracks token consumption across API calls.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// add accumulates another usage record into u.
func (u *TokenUsage) add(other *TokenUsage) {
	if other == nil {
		return
	}
	u.PromptTokens += other.PromptTokens
	u.CompletionTokens += other.CompletionTokens
	u.TotalTokens += other.TotalTokens
}

type ToolCallHandler func(name string, args json.RawMessage) (string, error)

// NewClient creates an AI client. compressTokenThreshold is the input prompt token
// count that triggers conversation compression; <= 0 disables compression.
func NewClient(apiKey, model, baseURL string, compressTokenThreshold int) *Client {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	// Build tool parameters at initialization
	toolDefs := ToolDefinitions()
	tools := make([]openai.ChatCompletionToolParam, len(toolDefs))
	for i, def := range toolDefs {
		// Build parameter properties
		properties := make(map[string]interface{})
		var required []string
		for _, p := range def.Parameters {
			properties[p.Name] = map[string]interface{}{
				"type":        p.Type,
				"description": p.Description,
			}
			if p.Required {
				required = append(required, p.Name)
			}
		}

		// Build parameters
		parameters := map[string]interface{}{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			parameters["required"] = required
		}

		tools[i] = openai.ChatCompletionToolParam{
			Type: constant.Function("function"),
			Function: shared.FunctionDefinitionParam{
				Name:        def.Name,
				Description: param.NewOpt(def.Description),
				Parameters:  openai.FunctionParameters(parameters),
			},
		}
	}

	return &Client{
		client:                 openai.NewClient(opts...),
		model:                  model,
		tools:                  tools,
		compressTokenThreshold: compressTokenThreshold,
	}
}

// Chat executes a multi-turn conversation with tool calls
func (c *Client) Chat(ctx context.Context, systemPrompt string, userMessage string, handler ToolCallHandler) (string, *TokenUsage, error) {
	return c.ChatWithLimit(ctx, systemPrompt, userMessage, handler, 20)
}

// ChatWithLimit executes a multi-turn conversation with a custom max iteration limit.
// Returns the last response text and cumulative token usage.
func (c *Client) ChatWithLimit(ctx context.Context, systemPrompt string, userMessage string, handler ToolCallHandler, maxIterations int) (string, *TokenUsage, error) {
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userMessage),
	}

	return c.chatLoop(ctx, messages, handler, maxIterations)
}

const (
	// fallbackCompressMessageThreshold is used only when the API does not report usage
	fallbackCompressMessageThreshold = 25
)

func (c *Client) chatLoop(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, handler ToolCallHandler, maxIterations int) (string, *TokenUsage, error) {
	var lastResponse string
	usage := &TokenUsage{}

	// 上一次成功请求的输入 prompt token 数（非累计值），用于判断压缩时机
	var lastPromptTokens int
	var lastUsageAvailable bool
	var justCompressed bool
	var compressionIneffective bool

	for i := 0; i < maxIterations; i++ {
		// 检查是否需要压缩对话历史（按上一次请求的实际 prompt token 数）
		if c.compressTokenThreshold > 0 && len(messages) > 3 {
			tokenTrigger := lastUsageAvailable && lastPromptTokens >= c.compressTokenThreshold
			fallbackTrigger := !lastUsageAvailable && len(messages) > fallbackCompressMessageThreshold

			switch {
			case tokenTrigger && justCompressed:
				// 压缩后仍超阈值，说明 system prompt + 初始消息本身已超限
				slog.Warn("prompt tokens still above threshold after compression, disabling further compression",
					"prompt_tokens", lastPromptTokens, "threshold", c.compressTokenThreshold)
				compressionIneffective = true
				justCompressed = false
			case tokenTrigger && !compressionIneffective:
				slog.Info("prompt token threshold reached, compressing conversation",
					"prompt_tokens", lastPromptTokens, "threshold", c.compressTokenThreshold,
					"message_count", len(messages))
				compressed, compressUsage, err := c.compressMessages(ctx, messages)
				if err != nil {
					return lastResponse, usage, fmt.Errorf("conversation compression failed: %w", err)
				}
				messages = compressed
				usage.add(compressUsage)
				justCompressed = true
				slog.Info("compression completed", "message_count", len(messages))
			case fallbackTrigger:
				slog.Info("prompt usage unavailable, compressing by message count",
					"message_count", len(messages), "threshold", fallbackCompressMessageThreshold)
				compressed, compressUsage, err := c.compressMessages(ctx, messages)
				if err != nil {
					return lastResponse, usage, fmt.Errorf("conversation compression failed: %w", err)
				}
				messages = compressed
				usage.add(compressUsage)
				justCompressed = false
				slog.Info("compression completed", "message_count", len(messages))
			default:
				justCompressed = false
			}
		}

		resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    c.model,
			Messages: messages,
			Tools:    c.tools,
		})
		if err != nil {
			return "", usage, fmt.Errorf("chat completion failed: %w", err)
		}

		// Accumulate token usage
		usage.PromptTokens += int(resp.Usage.PromptTokens)
		usage.CompletionTokens += int(resp.Usage.CompletionTokens)
		usage.TotalTokens += int(resp.Usage.TotalTokens)

		// 记录本次请求的输入 prompt 大小，供下一轮压缩判断使用
		promptTokens := int(resp.Usage.PromptTokens)
		lastUsageAvailable = promptTokens > 0
		if lastUsageAvailable {
			lastPromptTokens = promptTokens
		}
		slog.Debug("chat completion usage",
			"prompt_tokens", promptTokens,
			"completion_tokens", int(resp.Usage.CompletionTokens),
			"message_count", len(messages))

		if len(resp.Choices) == 0 {
			return "", usage, fmt.Errorf("no response choices")
		}

		choice := resp.Choices[0]

		// 添加 assistant 消息
		messages = append(messages, choice.Message.ToParam())

		// 保存最后的文本响应
		if choice.Message.Content != "" {
			lastResponse = choice.Message.Content
		}

		// 如果没有工具调用，追加提示词引导 AI 继续工作
		if len(choice.Message.ToolCalls) == 0 {
			slog.Info("no tool calls, continue conversation")
			messages = append(messages, openai.UserMessage("请继续你的审核工作。如果已完成所有审核，请调用 FinishReview 工具结束。"))
			continue
		}

		// 处理工具调用
		finishRequested := false
		for _, toolCall := range choice.Message.ToolCalls {
			slog.Info("tool call", "name", toolCall.Function.Name, "args", toolCall.Function.Arguments)

			// 检查是否调用了 FinishReview
			if toolCall.Function.Name == "FinishReview" {
				finishRequested = true
			}

			result, err := handler(toolCall.Function.Name, json.RawMessage(toolCall.Function.Arguments))
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}

			// 添加工具结果消息
			messages = append(messages, openai.ToolMessage(result, toolCall.ID))
		}

		// 如果调用了 FinishReview，结束对话
		if finishRequested {
			slog.Info("FinishReview called, ending conversation")
			return lastResponse, usage, nil
		}
	}

	return lastResponse, usage, nil
}

// compressMessages compresses conversation history
// Keeps system prompt (first message), compresses all other messages into a summary
func (c *Client) compressMessages(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) ([]openai.ChatCompletionMessageParamUnion, *TokenUsage, error) {
	if len(messages) <= 3 {
		return messages, nil, nil
	}

	// Extract messages to compress (all except system prompt and initial user message)
	messagesToCompress := messages[2:]

	compressSystemPrompt := `你是一个代码审核助手的对话压缩器。请将以下对话历史压缩为结构化摘要，供后续审核继续使用。

## 压缩原则
- 保留：审核结论、问题发现、进度状态等可执行信息
- 丢弃：工具调用细节、文件完整内容、搜索结果原文、中间推理过程

## 输出格式（严格遵守）
用中文，按以下格式输出：

### 审核进度
- 已审核：X 个文件（列出文件名）
- 待审核：X 个文件（列出文件名）

### 发现的问题
按文件分组列出，每个问题包含：文件名、行号、问题类型（bug/安全/风格/性能）、简要描述

### 已提交的评论
- 行级评论：X 条
- 整体评论：X 条

### 待办事项
如有未完成的工作，逐条列出；无则写"无"

只输出摘要内容，不要添加额外说明。`

	// Pass compression prompt + raw conversation history directly to AI
	compressMsgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(messagesToCompress)+2)
	compressMsgs = append(compressMsgs, openai.SystemMessage(compressSystemPrompt))
	compressMsgs = append(compressMsgs, messagesToCompress...)

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    c.model,
		Messages: compressMsgs,
	})
	if err != nil {
		slog.Error("conversation compression failed", "error", err)
		return nil, nil, err
	}

	compressUsage := &TokenUsage{
		PromptTokens:     int(resp.Usage.PromptTokens),
		CompletionTokens: int(resp.Usage.CompletionTokens),
		TotalTokens:      int(resp.Usage.TotalTokens),
	}

	summary := resp.Choices[0].Message.Content
	slog.Info("conversation compression completed", "summary_length", len(summary))

	// Build compressed message list: system prompt + initial message + summary
	compressed := []openai.ChatCompletionMessageParamUnion{
		messages[0],
		messages[1],
		openai.UserMessage(fmt.Sprintf("[对话历史已压缩]\n\n%s\n\n请继续审核工作。", summary)),
	}
	return compressed, compressUsage, nil
}

// FormatInitialMessage formats the initial review message
func FormatInitialMessage(mrTitle, mrDescription, sourceBranch, targetBranch, changesSummary string) string {
	return fmt.Sprintf(`## Merge Request 审核请求

**标题:** %s
**源分支:** %s → %s

**描述:**
%s

**变更内容:**
%s

---

请开始审核。`,
		mrTitle, sourceBranch, targetBranch, mrDescription, changesSummary)
}
