package builtin

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/shencheng/GopherClaw/internal/tool"
)

func EditFile(safety *tool.Safety) tool.HandlerFunc {
	return func(_ context.Context, input map[string]any) (string, error) {
		if safety == nil {
			return "", fmt.Errorf("safety is not initialized")
		}

		filePath, err := getRequiredString(input, "file_path")
		if err != nil {
			return "", err
		}
		oldString, err := getRequiredString(input, "old_string")
		if err != nil {
			return "", err
		}
		newString, err := getRequiredStringAllowEmpty(input, "new_string")
		if err != nil {
			return "", err
		}

		target, err := safety.SafePath(filePath)
		if err != nil {
			return "", err
		}

		contentBytes, err := os.ReadFile(target)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("File not found: %s", filePath)
			}
			return "", err
		}
		content := string(contentBytes)

		count := strings.Count(content, oldString)
		if count == 0 {
			return "", fmt.Errorf("old_string not found in file. Make sure it matches exactly")
		}
		if count > 1 {
			return "", fmt.Errorf("old_string found %d times. It must be unique. Provide more surrounding context", count)
		}

		newContent := strings.Replace(content, oldString, newString, 1)
		if err := os.WriteFile(target, []byte(newContent), 0o644); err != nil {
			return "", err
		}

		return fmt.Sprintf("Successfully edited %s", filePath), nil
	}
}
