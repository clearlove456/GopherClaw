package chat

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/shencheng/GopherClaw/internal/model"
)

type Loop struct {
	modelID      string
	systemPrompt string
	maxTokens    int
	client       model.ChatClient
}

func NewLoop(modelID, systemPrompt string, maxTokens int, client model.ChatClient) *Loop {
	return &Loop{
		modelID:      modelID,
		systemPrompt: systemPrompt,
		maxTokens:    maxTokens,
		client:       client,
	}
}

func (l *Loop) Run(ctx context.Context) error {
	messages := make([]model.Message, 0, 32)

	printInfo(strings.Repeat("=", 60))
	printInfo("  GopherClaw  |  Section 01: Agent 循环")
	printInfo(fmt.Sprintf("  Model: %s", l.modelID))
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

		messages = append(messages, model.Message{
			Role:    "user",
			Content: userInput,
		})

		result, callErr := l.client.Create(ctx, model.Request{
			ModelID:      l.modelID,
			SystemPrompt: l.systemPrompt,
			MaxTokens:    l.maxTokens,
			Messages:     messages,
		})
		if callErr != nil {
			printAPIError(callErr)
			messages = messages[:len(messages)-1]
			continue
		}

		switch result.StopReason {
		case "end_turn":
			printAssistant(result.AssistantText)
			messages = append(messages, model.Message{
				Role:    "assistant",
				Content: result.AssistantText,
			})
		case "tool_use":
			printInfo("[stop_reason=tool_use] 本节没有可用工具.")
			printInfo("下一节将加入工具支持.")
			messages = append(messages, model.Message{
				Role:    "assistant",
				Content: result.AssistantText,
			})
		default:
			printInfo(fmt.Sprintf("[stop_reason=%s]", result.StopReason))
			if strings.TrimSpace(result.AssistantText) != "" {
				printAssistant(result.AssistantText)
			}
			messages = append(messages, model.Message{
				Role:    "assistant",
				Content: result.AssistantText,
			})
		}
	}
}
