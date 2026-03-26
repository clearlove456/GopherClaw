package app

import (
	"context"
	"fmt"
	"os"

	"github.com/shencheng/GopherClaw/internal/chat"
	"github.com/shencheng/GopherClaw/internal/config"
	"github.com/shencheng/GopherClaw/internal/model"
)

func Run() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	client := model.NewOpenAICompatClient(cfg.APIKey, cfg.BaseURL)
	loop := chat.NewLoop(cfg.ModelID, cfg.SystemPrompt, cfg.MaxTokens, client)

	if err := loop.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		return 1
	}

	return 0
}
