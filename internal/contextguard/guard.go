package contextguard

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shencheng/GopherClaw/internal/model"
)

type Guard struct {
	MaxTokens int
}

func NewGuard(maxTokens int) *Guard {
	return &Guard{
		MaxTokens: maxTokens,
	}
}

// EstimateTokens 使用启发式
func (g *Guard) EstimateTokens(text string) int {
	return len(text) / 4
}

func (g *Guard) EstimateMessagesTokens(messages []model.Message) int {
	sum := 0
	for _, message := range messages {
		sum += g.EstimateTokens(message.Role)
		sum += g.EstimateTokens(message.ToolCallID)

		switch c := message.Content.(type) {
		case nil:
			// no-op
		case string:
			sum += g.EstimateTokens(c)
		default:
			// 对结构化 content 做保守估算（先序列化为 JSON 再估算）
			bytes, err := json.Marshal(c)
			if err == nil {
				sum += g.EstimateTokens(string(bytes))
			}
		}

		bytes, err := json.Marshal(message.ToolCalls)
		if err == nil {
			sum += g.EstimateTokens(string(bytes))
		}

	}
	return sum
}

func (g *Guard) TruncateLargeToolMessages(messages []model.Message, maxFraction float64) []model.Message {
	// clamp maxFraction 到合理范围，避免阈值异常
	if maxFraction <= 0 {
		maxFraction = 0.3
	}
	if maxFraction > 1 {
		maxFraction = 1
	}

	maxChars := int(float64(g.MaxTokens*4) * maxFraction)
	if maxChars <= 0 {
		maxChars = 2000
	}

	out := make([]model.Message, 0, len(messages))

	for _, m := range messages {
		if m.Role != "tool" {
			out = append(out, m)
			continue
		}

		if _, ok := m.Content.(string); !ok {
			out = append(out, m)
			continue
		}

		content := m.Content.(string)
		runes := []rune(content)
		if len(runes) <= maxChars {
			out = append(out, m)
			continue
		}

		// UTF-8 安全截断：按 rune 数量取前缀，再在前缀中找换行边界。
		prefix := string(runes[:maxChars])
		cut := strings.LastIndex(prefix, "\n")
		if cut <= 0 {
			cut = len(prefix)
		}

		head := prefix[:cut]
		tail := fmt.Sprintf("\n\n[... truncated (%d chars total, showing first %d) ...]", len(runes), len([]rune(head)))
		newContent := head + tail

		newMsg := m
		newMsg.Content = newContent
		out = append(out, newMsg)
	}

	return out
}

func IsContextOverflow(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	// 第一层：强特征，匹配到基本可以判定是上下文超限
	strongKeywords := []string{
		"context length exceeded",
		"maximum context length",
		"context_length_exceeded",
		"too many tokens",
		"maximum tokens",
		"prompt is too long",
		"input is too long",
		"token limit exceeded",
	}

	for _, kw := range strongKeywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}

	// 第二层：弱特征组合，减少误判
	if strings.Contains(msg, "context") {
		if strings.Contains(msg, "length") ||
			strings.Contains(msg, "window") ||
			strings.Contains(msg, "exceed") ||
			strings.Contains(msg, "limit") {
			return true
		}
	}

	if strings.Contains(msg, "token") {
		if strings.Contains(msg, "length") ||
			strings.Contains(msg, "exceed") ||
			strings.Contains(msg, "limit") ||
			strings.Contains(msg, "maximum") {
			return true
		}
	}

	return false
}

const toolMaxLen = 500

// serializeMessages 序列化辅助函数
/**        example
user: 帮我写一个 Go HTTP server
assistant: 好的，这是一个简单的示例...
tool: TOOL_OUTPUT_TOOL_OUTPUT_TOOL_OUTPUT_...（最多500字符）...(truncated)
*/
func serializeMessages(messages []model.Message) string {
	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString(m.Role)
		sb.WriteString(": ")

		// 2.content processing
		switch v := m.Content.(type) {
		case string:
			if m.Role == "tool" {
				sb.WriteString(truncate(v, toolMaxLen))
			} else {
				sb.WriteString(v)
			}
		case []byte:
			str := string(v)
			if m.Role == "tool" {
				sb.WriteString(truncate(str, toolMaxLen))
			} else {
				sb.WriteString(str)
			}
		default:
			// fallback：转字符串
			sb.WriteString(fmt.Sprintf("%v", v))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "...(truncated)"
}

func (g *Guard) CompactHistory(ctx context.Context, client model.ChatClient, req model.Request) ([]model.Message, error) {
	total := len(req.Messages)
	// 消息太少时压缩收益不大，直接返回。
	if total <= 8 {
		return req.Messages, nil
	}

	// keepCount: 最近消息保留数量（至少 6 条，或总量的 20%）。
	// 这些消息不参与压缩，保证“最近上下文”完整。
	keepCount := max(6, int(float64(total)*0.2))

	// compressCount: 计划压缩的旧消息数量（目标是总量的 50%）。
	compressCount := int(float64(total) * 0.5)

	// maxCompress: 实际最多可压缩多少条。
	// 不能把要保留的 recent 消息挤掉，所以最大压缩量 = total - keepCount。
	maxCompress := total - keepCount

	// 连 2 条都压不了，说明没有足够旧历史可压缩。
	if maxCompress < 2 {
		return req.Messages, nil
	}

	// 把目标压缩量夹到合法区间内：
	// - 上限：maxCompress（不能侵占 recent 区）
	// - 下限：2（压缩至少要有点意义）
	if compressCount > maxCompress {
		compressCount = maxCompress
	}
	if compressCount < 2 {
		return req.Messages, nil
	}

	cut := findSafeCutoff(req.Messages, compressCount)
	if cut < 2 || cut >= total {
		return req.Messages, nil
	}
	oldMessages := req.Messages[:cut]
	recentMessages := req.Messages[cut:]

	oldText := serializeMessages(oldMessages)
	summaryPrompt := "Summarize the following conversation concisely, preserving key facts and decisions. Output only the summary.\n\n" + oldText

	maxSummaryTokens := req.MaxTokens
	if maxSummaryTokens <= 0 || maxSummaryTokens > 1024 {
		maxSummaryTokens = 1024
	}

	summaryResp, err := client.Create(ctx, model.Request{
		ModelID:      req.ModelID,
		SystemPrompt: "You are a concise conversation summarizer. Preserve key facts and decisions.",
		MaxTokens:    maxSummaryTokens,
		Messages: []model.Message{
			{
				Role:    "user",
				Content: summaryPrompt,
			},
		},
		Tools: nil,
	})
	if err != nil {
		if len(recentMessages) > 0 {
			return recentMessages, nil
		}
		return req.Messages, nil
	}

	summaryText := strings.TrimSpace(summaryResp.AssistantText)
	if summaryText == "" {
		if len(recentMessages) > 0 {
			return recentMessages, nil
		}
		return req.Messages, nil
	}

	compacted := make([]model.Message, 0, 2+len(recentMessages))
	compacted = append(compacted, model.Message{
		Role:    "user",
		Content: "[Previous conversation summary]\n" + summaryText,
	})
	compacted = append(compacted, model.Message{
		Role:    "assistant",
		Content: "Understood. I will use this summary as context.",
	})
	compacted = append(compacted, recentMessages...)

	return compacted, nil
}

func findSafeCutoff(messages []model.Message, target int) int {
	if len(messages) == 0 {
		return 0
	}
	if target <= 0 {
		return 0
	}
	if target >= len(messages) {
		return len(messages) - 1
	}

	cut := target

	// 尽量不要从 tool 消息开始切分，避免把工具链切断。
	for cut > 1 && cut < len(messages) && messages[cut].Role == "tool" {
		cut--
	}

	// 尽量把 cut 落在 user 边界。
	for cut > 1 && messages[cut].Role != "user" {
		cut--
	}

	return cut
}

func (g *Guard) Call(ctx context.Context, client model.ChatClient, req model.Request, maxRetries int) (model.Result, []model.Message, error) {
	currentMessages := append([]model.Message(nil), req.Messages...)
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err := client.Create(ctx, model.Request{
			ModelID:      req.ModelID,
			SystemPrompt: req.SystemPrompt,
			MaxTokens:    req.MaxTokens,
			Messages:     currentMessages,
			Tools:        req.Tools,
		})

		if err == nil {
			return result, currentMessages, nil
		}

		lastErr = err
		if !IsContextOverflow(err) || attempt >= maxRetries {
			break
		}

		// 如果是第一次的话先阶段工具的调用命令
		if attempt == 0 {
			currentMessages = g.TruncateLargeToolMessages(currentMessages, 0.3)
			continue
		}

		// 第二次还是溢出的话压缩上下文
		if attempt == 1 {
			compacted, compactErr := g.CompactHistory(ctx, client, model.Request{
				ModelID:      req.ModelID,
				SystemPrompt: req.SystemPrompt,
				MaxTokens:    req.MaxTokens,
				Messages:     currentMessages,
				Tools:        req.Tools,
			})
			if compactErr != nil {
				return model.Result{}, currentMessages, fmt.Errorf("compact history failed: %w", compactErr)
			}
			if len(compacted) == 0 {
				return model.Result{}, currentMessages, fmt.Errorf("compact history failed: empty compacted messages")
			}
			currentMessages = compacted
			continue
		}
	}
	return model.Result{}, currentMessages, lastErr
}
