package app

import (
	"context"
	"fmt"
	"os"

	"github.com/shencheng/GopherClaw/internal/chat"
	"github.com/shencheng/GopherClaw/internal/config"
	"github.com/shencheng/GopherClaw/internal/contextguard"
	"github.com/shencheng/GopherClaw/internal/model"
	"github.com/shencheng/GopherClaw/internal/session"
	"github.com/shencheng/GopherClaw/internal/tool"
	"github.com/shencheng/GopherClaw/internal/tool/builtin"
)

func Run() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	client := model.NewOpenAICompatClient(cfg.APIKey, cfg.BaseURL)

	workdir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve working directory failed: %v\n", err)
		return 1
	}

	safety, err := tool.NewSafety(workdir, tool.DefaultMaxToolOutput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialize tool safety failed: %v\n", err)
		return 1
	}

	registry := tool.NewRegistry()
	builtin.RegisterAll(registry, safety)
	dispatcher := tool.NewDispatcher(registry)
	toolSchemas := tool.Schemas()

	store, err := session.NewSessionStore("claw0", workdir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialize session store failed: %v\n", err)
		return 1
	}

	guard := contextguard.NewGuard(180000)

	loop := chat.NewLoop(
		cfg.ModelID,
		cfg.SystemPrompt,
		cfg.MaxTokens,
		client,
		dispatcher,
		toolSchemas,
		store,
		guard,
	)

	if err := loop.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		return 1
	}

	return 0
}
