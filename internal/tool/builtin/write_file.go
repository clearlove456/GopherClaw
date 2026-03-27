package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shencheng/GopherClaw/internal/tool"
)

func WriteFile(safety *tool.Safety) tool.HandlerFunc {
	return func(_ context.Context, input map[string]any) (string, error) {
		if safety == nil {
			return "", fmt.Errorf("safety is not initialized")
		}

		filePath, err := getRequiredString(input, "file_path")
		if err != nil {
			return "", err
		}
		content, err := getRequiredStringAllowEmpty(input, "content")
		if err != nil {
			return "", err
		}

		target, err := safety.SafePath(filePath)
		if err != nil {
			return "", err
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return "", err
		}

		return fmt.Sprintf("Successfully wrote %d chars to %s", len(content), filePath), nil
	}
}
