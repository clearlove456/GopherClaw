package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/shencheng/GopherClaw/internal/contextguard"
	"github.com/shencheng/GopherClaw/internal/model"
	"github.com/shencheng/GopherClaw/internal/session"
	"github.com/shencheng/GopherClaw/internal/tool"
)

type Loop struct {
	modelID      string
	systemPrompt string
	maxTokens    int
	client       model.ChatClient
	dispatcher   *tool.Dispatcher
	tools        []model.ToolSchema
	store        *session.SessionStore
	guard        *contextguard.Guard
}

func NewLoop(
	modelID,
	systemPrompt string,
	maxTokens int,
	client model.ChatClient,
	dispatcher *tool.Dispatcher,
	tools []model.ToolSchema,
	store *session.SessionStore,
	guard *contextguard.Guard,
) *Loop {
	return &Loop{
		modelID:      modelID,
		systemPrompt: systemPrompt,
		maxTokens:    maxTokens,
		client:       client,
		dispatcher:   dispatcher,
		tools:        tools,
		store:        store,
		guard:        guard,
	}
}

func (l *Loop) Run(ctx context.Context) error {
	messages := make([]model.Message, 0, 32)

	sessionEnabled := l.store != nil
	if sessionEnabled {
		sessions := l.store.ListSessions()
		if len(sessions) > 0 {
			loaded, err := l.store.LoadSession(sessions[0].ID)
			if err != nil {
				return err
			}
			messages = append(messages, loaded...)
		} else {
			if _, err := l.store.CreateSession("initial"); err != nil {
				return fmt.Errorf("create initial session: %w", err)
			}
		}
	}

	printInfo(strings.Repeat("=", 60))
	printInfo(fmt.Sprintf("  Model: %s", l.modelID))
	printInfo(fmt.Sprintf("  Tools: %s", strings.Join(toolNames(l.tools), ", ")))
	printInfo("  输入 'quit' 或 'exit' 退出. Ctrl+C 同样有效.")
	printInfo(strings.Repeat("=", 60))
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(coloredPrompt())
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("\n%s再见.%s\n", dim, reset)
			return nil
		}

		userInput := strings.TrimSpace(input)
		if userInput == "" {
			continue
		}

		lower := strings.ToLower(userInput)
		if lower == "quit" || lower == "exit" {
			fmt.Printf("%s再见.%s\n", dim, reset)
			return nil
		}

		if strings.HasPrefix(userInput, "/") {
			ok, err := l.handleCommand(userInput, &messages)
			if err != nil {
				return err
			}
			if ok {
				continue
			}
		}

		turnStart := len(messages)
		messages = append(messages, model.Message{
			Role:    "user",
			Content: userInput,
		})

		userPersisted := false

		for {
			result, guardedMessages, callErr := l.guard.Call(ctx, l.client, model.Request{
				ModelID:      l.modelID,
				SystemPrompt: l.systemPrompt,
				MaxTokens:    l.maxTokens,
				Messages:     messages,
				Tools:        l.tools,
			}, 2)
			if callErr != nil {
				printAPIError(callErr)
				messages = messages[:turnStart]
				break
			}
			messages = guardedMessages

			if sessionEnabled && !userPersisted {
				if err := l.store.SaveTurn("user", userInput); err != nil {
					return fmt.Errorf("save user turn: %w", err)
				}
				userPersisted = true
			}

			messages = append(messages, result.AssistantMessage)

			switch result.StopReason {
			case "tool_use":
				if len(result.ToolCalls) == 0 {
					printInfo("[stop_reason=tool_use] no tool call payload returned.")
					break
				}

				// 批量回传
				for _, tc := range result.ToolCalls {
					printTool(tc.Name, summarizeToolArgs(tc.Arguments))
					output := l.dispatcher.Process(ctx, tc.Name, tc.Arguments)

					if sessionEnabled {
						if err := l.store.SaveToolResult(tc.ID, tc.Name, tc.Arguments, output); err != nil {
							return fmt.Errorf("save tool result: %w", err)
						}
					}

					messages = append(messages, model.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    output,
					})
				}
				continue

			case "end_turn":
				if strings.TrimSpace(result.AssistantText) != "" {
					printAssistant(result.AssistantText)
				}

				if sessionEnabled {
					if err := l.store.SaveTurn("assistant", result.AssistantText); err != nil {
						return fmt.Errorf("save assistant turn: %w", err)
					}
				}
				break

			default:
				printInfo(fmt.Sprintf("[stop_reason=%s]", result.StopReason))
				if strings.TrimSpace(result.AssistantText) != "" {
					printAssistant(result.AssistantText)
				}
				break
			}

			break
		}
	}
}

func (l *Loop) handleCommand(input string, messages *[]model.Message) (bool, error) {
	parts := strings.SplitN(strings.TrimSpace(input), " ", 2)
	cmd := strings.ToLower(parts[0])
	arg := ""
	if len(parts) == 2 {
		arg = strings.TrimSpace(parts[1])
	}

	if l.store == nil {
		printInfo("session store is disabled.")
		return true, nil
	}

	switch cmd {
	case "/new":
		sid, err := l.store.CreateSession(arg)
		if err != nil {
			return false, err
		}
		// clear messages
		*messages = (*messages)[:0]

		if arg == "" {
			printInfo(fmt.Sprintf("new session created: %s", sid))
		} else {
			printInfo(fmt.Sprintf("new session created: %s (%s)", sid, arg))
		}

		return true, nil
	case "/list":
		sessions := l.store.ListSessions()
		if len(sessions) == 0 {
			printInfo("no sessions found")
			return true, nil
		}

		for _, s := range sessions {
			current := ""
			if s.ID == l.store.CurrentSessionID {
				current = "  <-- current"
			}
			label := s.Meta.Label
			if label == "" {
				label = "-"
			}
			fmt.Printf("%s  label=%s  msgs=%d  last=%s%s\n",
				s.ID, label, s.Meta.MessageCount, s.Meta.LastActive, current)
		}
		return true, nil

	case "/switch":
		if arg == "" {
			printInfo("usage: /switch <session_id_prefix>")
			return true, nil
		}

		matches := make([]string, 0, 4)
		sessions := l.store.ListSessions()
		for _, s := range sessions {
			if strings.HasPrefix(s.ID, arg) {
				matches = append(matches, s.ID)
			}
		}

		if len(matches) == 0 {
			printInfo(fmt.Sprintf("session not found: %s", arg))
			return true, nil
		}
		if len(matches) > 1 {
			printInfo("ambiguous session prefix, matches:")
			for _, id := range matches {
				fmt.Printf("  %s\n", id)
			}
			return true, nil
		}

		loaded, err := l.store.LoadSession(matches[0])
		if err != nil {
			return false, err
		}
		*messages = loaded
		printInfo(fmt.Sprintf("switched to session: %s (%d messages)", matches[0], len(loaded)))
		return true, nil

	default:
		printInfo(fmt.Sprintf("unknown command: %s", cmd))
		printInfo("available commands: /new [label], /list, /switch <session_id_prefix>")
		return true, nil
	}
}

func toolNames(schemas []model.ToolSchema) []string {
	names := make([]string, 0, len(schemas))
	for _, schema := range schemas {
		if strings.TrimSpace(schema.Function.Name) == "" {
			continue
		}
		names = append(names, schema.Function.Name)
	}
	return names
}

func summarizeToolArgs(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}

	encoded, err := json.Marshal(args)
	if err != nil {
		return "[invalid tool args]"
	}

	text := string(encoded)
	if len(text) <= 120 {
		return text
	}

	return text[:120] + "...(truncated)"
}
