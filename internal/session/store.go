package session

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shencheng/GopherClaw/internal/model"
)

// SessionMeta 表示一个会话在索引文件中的元信息。
// 索引文件（sessions.json）只存“会话概览”，不存完整消息内容。
type SessionMeta struct {
	Label        string `json:"label"`       // 会话标签，可为空
	CreatedAt    string `json:"created_at"`  // 会话创建时间（RFC3339 UTC）
	LastActive   string `json:"last_active"` // 最近活跃时间（RFC3339 UTC）
	MessageCount int    `json:"message_count"`
}

// SessionInfo 是对外返回的会话列表项。
// 比 SessionMeta 多一个 ID，方便 UI/REPL 展示与切换。
type SessionInfo struct {
	ID   string
	Meta SessionMeta
}

// SessionStore 负责会话持久化与重建，核心特性：
// 1. 会话索引：sessions.json
// 2. 会话正文：每个会话一个 jsonl 文件（按行追加）
// 3. 线程安全：通过 RWMutex 保护内存索引和当前会话指针
type SessionStore struct {
	AgentID          string // 逻辑 agent ID（例如 claw0）
	BaseDir          string // *.jsonl 文件所在目录
	IndexPath        string // sessions.json 路径
	CurrentSessionID string // 当前会话 ID（空表示尚未选择）

	mu    sync.RWMutex
	Index map[string]*SessionMeta // sessionID -> meta
}

// NewSessionStore 创建并初始化会话存储。
// - 若 agentID 为空，回退到 default
// - 若 workspaceDir 为空，使用当前工作目录
// - 自动创建目录并加载索引文件
func NewSessionStore(agentID, workspaceDir string) (*SessionStore, error) {
	if strings.TrimSpace(agentID) == "" {
		agentID = "default"
	}
	if strings.TrimSpace(workspaceDir) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		workspaceDir = wd
	}

	// 目录结构：
	// .sessions/agents/<agentID>/sessions/*.jsonl
	// .sessions/agents/<agentID>/sessions.json
	baseDir := filepath.Join(workspaceDir, ".sessions", "agents", agentID, "sessions")
	indexPath := filepath.Join(workspaceDir, ".sessions", "agents", agentID, "sessions.json")

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}

	store := &SessionStore{
		AgentID:   agentID,
		BaseDir:   baseDir,
		IndexPath: indexPath,
		Index:     make(map[string]*SessionMeta),
	}

	if err := store.loadIndex(); err != nil {
		return nil, err
	}

	return store, nil
}

// GenerateID 生成 12 位十六进制会话 ID（6 字节随机数）。
func GenerateID() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// CreateSession 创建新会话并切换为当前会话。
// 流程：
// 1. 生成 sessionID
// 2. 更新内存索引
// 3. 持久化 sessions.json
// 4. 创建空 jsonl 文件
func (s *SessionStore) CreateSession(label string) (string, error) {
	sessionID, err := GenerateID()
	if err != nil {
		return "", err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	s.mu.Lock()
	s.Index[sessionID] = &SessionMeta{
		Label:        label,
		CreatedAt:    now,
		LastActive:   now,
		MessageCount: 0,
	}
	s.CurrentSessionID = sessionID
	s.mu.Unlock()

	if err := s.saveIndex(); err != nil {
		return "", err
	}

	file, err := os.Create(s.sessionPath(sessionID))
	if err != nil {
		return "", fmt.Errorf("create session file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close session file: %w", err)
	}

	return sessionID, nil
}

// LoadSession 从指定会话的 jsonl 文件重建消息历史。
// 注意：
// - 遇到损坏行会跳过，尽量“容错重建”
// - 若文件不存在，返回空历史，不视为错误
func (s *SessionStore) LoadSession(sessionID string) ([]model.Message, error) {
	path := s.sessionPath(sessionID)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return []model.Message{}, nil
		}
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open session file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	messages := make([]model.Message, 0, 128)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			// 单行损坏不影响整体恢复
			continue
		}

		restored := restoreMessage(record)
		if restored == nil {
			continue
		}
		messages = append(messages, *restored)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan session file: %w", err)
	}

	s.mu.Lock()
	s.CurrentSessionID = sessionID
	s.mu.Unlock()

	return messages, nil
}

// SaveTurn 保存普通回合消息（user / assistant / tool）。
// content 允许是 string 或任意可 JSON 序列化对象。
func (s *SessionStore) SaveTurn(role string, content any) error {
	if role == "" {
		return fmt.Errorf("empty role")
	}

	s.mu.RLock()
	sessionID := s.CurrentSessionID
	s.mu.RUnlock()
	if sessionID == "" {
		// 未选择会话时静默跳过，便于上层渐进接入
		return nil
	}

	return s.appendTranscript(sessionID, map[string]any{
		"type":    role,
		"content": content,
		"ts":      time.Now().Unix(),
	})
}

// SaveToolResult 保存一次工具调用链：
// - tool_use（工具请求）
// - tool_result（工具结果）
// 这样回放时可以复原工具执行上下文。
func (s *SessionStore) SaveToolResult(toolUseID, name string, toolInput map[string]any, result string) error {
	s.mu.RLock()
	sessionID := s.CurrentSessionID
	s.mu.RUnlock()
	if sessionID == "" {
		return nil
	}

	ts := time.Now().Unix()
	if err := s.appendTranscript(sessionID, map[string]any{
		"type":        "tool_use",
		"tool_use_id": toolUseID,
		"name":        name,
		"input":       toolInput,
		"ts":          ts,
	}); err != nil {
		return err
	}

	return s.appendTranscript(sessionID, map[string]any{
		"type":        "tool_result",
		"tool_use_id": toolUseID,
		"content":     result,
		"ts":          ts,
	})
}

// ListSessions 返回会话列表（按 LastActive 倒序）。
func (s *SessionStore) ListSessions() []SessionInfo {
	s.mu.RLock()
	items := make([]SessionInfo, 0, len(s.Index))
	for id, meta := range s.Index {
		if meta == nil {
			continue
		}
		items = append(items, SessionInfo{
			ID: id,
			Meta: SessionMeta{
				Label:        meta.Label,
				CreatedAt:    meta.CreatedAt,
				LastActive:   meta.LastActive,
				MessageCount: meta.MessageCount,
			},
		})
	}
	s.mu.RUnlock()

	sort.Slice(items, func(i, j int) bool {
		return items[i].Meta.LastActive > items[j].Meta.LastActive
	})
	return items
}

// appendTranscript 以 JSONL 方式追加一条记录，并更新 sessions.json 元数据。
func (s *SessionStore) appendTranscript(sessionID string, record map[string]any) error {
	path := s.sessionPath(sessionID)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open transcript file: %w", err)
	}
	defer file.Close()

	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal transcript: %w", err)
	}
	if _, err := file.WriteString(string(line) + "\n"); err != nil {
		return fmt.Errorf("append transcript: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	if meta, ok := s.Index[sessionID]; ok && meta != nil {
		meta.LastActive = now
		meta.MessageCount++
	}
	s.mu.Unlock()

	return s.saveIndex()
}

// loadIndex 读取 sessions.json 到内存。
// 文件不存在时视为“首次启动”，不报错。
func (s *SessionStore) loadIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.IndexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read index: %w", err)
	}

	index := make(map[string]*SessionMeta)
	if len(strings.TrimSpace(string(data))) == 0 {
		s.Index = index
		return nil
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("unmarshal index: %w", err)
	}
	s.Index = index
	return nil
}

// saveIndex 把内存索引写回 sessions.json。
func (s *SessionStore) saveIndex() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.Index, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.IndexPath), 0o755); err != nil {
		return fmt.Errorf("ensure index directory: %w", err)
	}

	if err := os.WriteFile(s.IndexPath, data, 0o644); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	return nil
}

// sessionPath 返回某个会话的 jsonl 路径。
func (s *SessionStore) sessionPath(sessionID string) string {
	return filepath.Join(s.BaseDir, sessionID+".jsonl")
}

// restoreMessage 把一条 JSONL record 转为 model.Message。
// 这里做“最小可恢复”策略：能恢复就恢复，未知类型返回 nil。
func restoreMessage(record map[string]any) *model.Message {
	typ, _ := record["type"].(string)
	switch typ {
	case "user", "assistant":
		return &model.Message{
			Role:    typ,
			Content: toString(record["content"]),
		}
	case "tool":
		return &model.Message{
			Role:       "tool",
			ToolCallID: toString(record["tool_call_id"]),
			Content:    toString(record["content"]),
		}
	case "tool_result":
		// 兼容历史格式：tool_result 中使用 tool_use_id
		return &model.Message{
			Role:       "tool",
			ToolCallID: toString(record["tool_use_id"]),
			Content:    toString(record["content"]),
		}
	case "tool_use":
		// tool_use 自身不是最终模型输入消息；
		// 这里兼容性地恢复为 assistant + tool_calls，便于后续排查和重放。
		id := toString(record["tool_use_id"])
		name := toString(record["name"])
		input, _ := record["input"].(map[string]any)
		if input == nil {
			input = map[string]any{}
		}
		return &model.Message{
			Role:    "assistant",
			Content: "",
			ToolCalls: []model.AssistantToolCall{
				{
					ID:   id,
					Type: "function",
					Function: model.ToolFunctionCall{
						Name:      name,
						Arguments: toJSON(input),
					},
				},
			},
		}
	default:
		return nil
	}
}

// toString 尝试把任意值转为字符串：
// - string 直接返回
// - 其他类型 JSON 序列化后返回
func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// toJSON 把任意值编码成 JSON 字符串。
func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
