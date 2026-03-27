package builtin

import (
	"context"
	"fmt"
	"os"

	"github.com/shencheng/GopherClaw/internal/tool"
)

func ReadFile(safety *tool.Safety) tool.HandlerFunc {
	return func(_ context.Context, input map[string]any) (string, error) {
		if safety == nil {
			return "", fmt.Errorf("safety is not initialized")
		}

		filePath, err := getRequiredString(input, "file_path")
		if err != nil {
			return "", err
		}

		target, err := safety.SafePath(filePath)
		if err != nil {
			return "", err
		}

		info, err := os.Stat(target)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("File not found: %s", filePath)
			}
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("Not a file: %s", filePath)
		}

		content, err := os.ReadFile(target)
		if err != nil {
			return "", err
		}

		return safety.Truncate(string(content)), nil
	}
}
