package model

import "context"

// Message 是发送给/接收自 OpenAI-compatible chat/completions 的消息单元。
//
// Section 02 里你会用到 3 种角色：
// - user: 用户输入
// - assistant: 模型回复（可能包含 tool_calls）
// - tool: 工具执行结果（要带 tool_call_id）
type Message struct {
	Role       string              `json:"role"`
	Content    any                 `json:"content"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
	ToolCalls  []AssistantToolCall `json:"tool_calls,omitempty"`
}

// AssistantToolCall 是 assistant 消息里的单个工具调用请求。
// 对应 OpenAI 的 message.tool_calls[].
type AssistantToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // 固定为 "function"
	Function ToolFunctionCall `json:"function"`
}

// ToolFunctionCall 是工具调用的函数部分。
// Arguments 是 JSON 字符串，后续要反序列化成 map[string]any 才能执行本地工具。
type ToolFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolSchema 是发送给模型的“工具定义”（告诉模型有哪些工具可用）。
type ToolSchema struct {
	Type     string         `json:"type"`
	Function ToolDefinition `json:"function"`
}

// ToolDefinition 是一个工具的元信息 + 参数 JSON Schema。
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall 是把 assistant 的 tool_calls 解析后，给本地调度器执行的结构。
type ToolCall struct {
	ID        string // tool_call_id
	Name      string
	Arguments map[string]any
}

// Request 是一次模型调用的输入。
type Request struct {
	ModelID      string
	SystemPrompt string
	MaxTokens    int
	Messages     []Message
	Tools        []ToolSchema
}

// Result 是一次模型调用的输出。
// - StopReason: end_turn / tool_use / other
// - AssistantText: 便捷字段，便于直接打印
// - AssistantMessage: 原始 assistant 消息，便于原样放回对话历史
// - ToolCalls: 解析后的可执行工具调用列表
type Result struct {
	StopReason       string // end_turn / tool_use / other
	AssistantText    string
	AssistantMessage Message
	ToolCalls        []ToolCall
}

// ChatClient 是模型客户端抽象，方便以后替换实现。
type ChatClient interface {
	Create(ctx context.Context, req Request) (Result, error)
}
