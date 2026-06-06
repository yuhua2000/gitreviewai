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
	client openai.Client
	model  string
	tools  []openai.ChatCompletionToolParam
}

// TokenUsage tracks token consumption across API calls.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type ToolCallHandler func(name string, args json.RawMessage) (string, error)

func NewClient(apiKey, model, baseURL string) *Client {
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
		client: openai.NewClient(opts...),
		model:  model,
		tools:  tools,
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
	// compressThreshold is the message count threshold to trigger compression
	compressThreshold = 25
)

func (c *Client) chatLoop(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, handler ToolCallHandler, maxIterations int) (string, *TokenUsage, error) {
	var lastResponse string
	usage := &TokenUsage{}

	for i := 0; i < maxIterations; i++ {
		// 检查是否需要压缩对话历史
		if len(messages) > compressThreshold {
			slog.Info("conversation history too long, compressing", "count", len(messages))
			compressed, err := c.compressMessages(ctx, messages)
			if err != nil {
				return lastResponse, usage, fmt.Errorf("conversation compression failed: %w", err)
			}
			messages = compressed
			slog.Info("compression completed", "count", len(messages))
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
func (c *Client) compressMessages(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) ([]openai.ChatCompletionMessageParamUnion, error) {
	if len(messages) <= 3 {
		return messages, nil
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
		return nil, err
	}

	summary := resp.Choices[0].Message.Content
	slog.Info("conversation compression completed", "summary_length", len(summary))

	// Build compressed message list: system prompt + initial message + summary
	compressed := []openai.ChatCompletionMessageParamUnion{
		messages[0],
		messages[1],
		openai.UserMessage(fmt.Sprintf("[对话历史已压缩]\n\n%s\n\n请继续审核工作。", summary)),
	}
	return compressed, nil
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
