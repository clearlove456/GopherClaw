# Section 03 方案: 会话持久化 + 上下文保护

## 1. 目标

在当前 `Section 02` 的基础上，增加两层能力：

1. `SessionStore`：把每一轮对话和工具调用结果持久化到本地（JSONL），支持重启后恢复。
2. `ContextGuard`：在上下文过长时自动降载（先截断工具结果，再压缩历史），尽量避免模型调用失败。

不改变主循环本质：仍然是外层用户循环 + 内层 tool-use 循环。

---

## 2. 与当前代码的对接点

当前关键文件：

- `internal/chat/loop.go`：主循环
- `internal/model/types.go`：消息结构
- `internal/model/openai_compat.go`：模型请求/响应
- `internal/app/app.go`：启动装配

Section 03 主要改动集中在：

1. 新增 `internal/session` 包（持久化）
2. 新增 `internal/contextguard` 包（上下文保护）
3. 调整 `internal/model.Message`，让 `content` 支持非纯文本（tool result / compact summary）
4. 在 `chat/loop.go` 注入 `store + guard`

---

## 3. 目录设计（建议）

```text
internal/
  session/
    store.go          # SessionStore: create/load/save/list
    types.go          # JSONL record 结构
  contextguard/
    guard.go          # Guard + overflow 重试策略
  chat/
    loop.go           # 接入 store + guard + REPL 命令
  model/
    types.go          # Message.Content 改为 any（或结构化 ContentPart）
```

可选：

- `workspace/.sessions/...` 放会话数据（避免污染项目根目录）

---

## 4. 数据格式设计

### 4.1 JSONL 记录（磁盘）

每行一个 JSON：

1. `user`
2. `assistant`
3. `tool_use`
4. `tool_result`

建议字段：

```json
{"type":"user","content":"...","ts":1710000000}
{"type":"assistant","content":"...","ts":1710000001}
{"type":"tool_use","tool_use_id":"call_x","name":"read_file","input":{"file_path":"README.md"},"ts":1710000002}
{"type":"tool_result","tool_use_id":"call_x","content":"...","ts":1710000002}
```

### 4.2 Session 索引

`sessions.json` 维护元数据：

- `label`
- `created_at`
- `last_active`
- `message_count`

---

## 5. Message 结构改造（必须）

你当前 `Message.Content` 是 `string`，Section 03 会碰到：

1. `role=tool` 时内容通常仍是文本（可用 string）
2. 但上下文压缩、历史重放时，可能需要结构化内容

建议改为：

```go
type Message struct {
    Role       string              `json:"role"`
    Content    any                 `json:"content"` // string 或 []map[string]any
    ToolCallID string              `json:"tool_call_id,omitempty"`
    ToolCalls  []AssistantToolCall `json:"tool_calls,omitempty"`
}
```

并在 `openai_compat.go` 发请求前做一次标准化：

- `string` 直接透传
- `[]map[string]any` 直接透传
- 其他类型直接报错（避免静默错）

---

## 6. SessionStore 设计

## 6.1 核心职责

1. `CreateSession(label)` 创建新会话
2. `LoadSession(id)` 从 JSONL 重建 `[]model.Message`
3. `SaveTurn(role, content)` 追加消息
4. `SaveToolResult(...)` 记录 tool_use/tool_result
5. `ListSessions()` 列表排序（按 `last_active`）

## 6.2 文件路径

建议默认路径：

`workspace/.sessions/agents/<agent_id>/sessions/<session_id>.jsonl`

索引：

`workspace/.sessions/agents/<agent_id>/sessions.json`

## 6.3 重建规则

重放 JSONL 时：

1. `user` -> `Message{Role:"user", Content:string}`
2. `assistant` -> `Message{Role:"assistant", Content:string}`
3. `tool_use/tool_result`  
  OpenAI 兼容模式下不必强行重建历史 `tool_use` 块到 `assistant.tool_calls`；  
  推荐最小实现：只重放 `user/assistant/tool` 三类消息，保证历史语义可读即可。

说明：这是你当前协议层最稳的版本，避免先把“历史工具调用结构化重放”复杂化。

---

## 7. ContextGuard 设计

## 7.1 触发条件

当模型调用报错字符串中包含 `context`/`token`/`length` 相关关键字时触发。

## 7.2 三阶段策略

1. 第 0 次：正常请求
2. 第 1 次：截断过大的 tool 输出消息（如单条超过 30% 上下文预算）
3. 第 2 次：压缩旧历史（前 50%）为摘要消息，再请求
4. 仍失败：返回错误给上层

## 7.3 token 估算

先用粗估：`len(text)/4`。  
后续如果接入 tokenizer 再替换。

---

## 8. chat/loop.go 改造点

当前 loop 改造为：

1. 启动时：
  - `store := session.NewStore("claw0", workdir)`
  - 恢复最近会话或创建新会话
2. 用户输入后：
  - 先 `messages append`
  - 同时 `store.SaveTurn("user", userInput)`
3. 调模型时：
  - 不直接 `client.Create`，改为 `guard.Call(...)`
4. `tool_use` 分支：
  - 执行工具
  - `store.SaveToolResult(...)`
  - 追加 `role=tool` 消息
5. `end_turn` 分支：
  - 打印文本
  - `store.SaveTurn("assistant", assistantText)`

---

## 9. REPL 命令（建议）

推荐先做这 5 个：

1. `/new [label]` 新建会话
2. `/list` 会话列表
3. `/switch <id-prefix>` 切换会话
4. `/context` 显示估算上下文占用
5. `/compact` 手动触发压缩

---

## 10. 分阶段实施顺序（务实版）

### Phase A（先跑通持久化）

1. 新建 `internal/session/store.go`
2. 实现 `Create/Load/Save/List`
3. loop 接入 `SaveTurn`（仅 user/assistant）
4. 验收：重启程序后可恢复上一轮对话

### Phase B（接入 tool 记录）

1. `SaveToolResult` 落盘
2. `tool_use` 分支写入记录
3. 验收：JSONL 能看到 tool_use/tool_result 对

### Phase C（上下文保护）

1. 新建 `internal/contextguard/guard.go`
2. 实现 `truncate tool result`
3. 实现 `compact history summary`（可先用简单截断摘要替代 LLM 摘要）
4. 验收：超长工具输出不再直接打爆上下文

### Phase D（REPL 命令）

1. `/new /list /switch`
2. `/context /compact`
3. 验收：会话管理可用

---

## 11. 验收清单

1. 正常对话后退出重启，历史存在。
2. 切换会话不串线。
3. 执行大输出工具（如 `ls -R`），程序不会立即因 context overflow 崩掉。
4. `go test ./...` 通过。
5. 手动看 JSONL，可读且按时间追加。

---

## 12. 风险与注意事项

1. 并发写入：当前 REPL 单进程问题不大，后续多进程要加文件锁。
2. 隐私：会话是明文 JSONL，必要时考虑脱敏或加密。
3. 压缩正确性：摘要会损失细节，必须保留最近若干轮原始消息。
4. 协议兼容：OpenAI-compatible 厂商字段可能略有差异，`tool_calls` 解析保持容错。

---

## 13. 你下一步可以直接开始写的最小任务

1. 先把 `model.Message.Content` 改成 `any`。
2. 新建 `internal/session/store.go`，先只实现：
  - `CreateSession`
  - `LoadSession`
  - `SaveTurn`
3. 在 `chat/loop.go` 接入这三处调用（不做 guard）。
4. 跑通后我再带你加 `ContextGuard`。

