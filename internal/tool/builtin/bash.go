package builtin

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/shencheng/GopherClaw/internal/tool"
)

func Bash(safety *tool.Safety) tool.HandlerFunc {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if safety == nil {
			return "", fmt.Errorf("safety is not initialized")
		}

		command, err := getRequiredString(input, "command")
		if err != nil {
			return "", err
		}
		timeoutSec, err := getOptionalInt(input, "timeout", 30)
		if err != nil {
			return "", err
		}
		if timeoutSec <= 0 {
			timeoutSec = 30
		}
		if tool.IsDangerousCommand(command) {
			return "", fmt.Errorf("Refused to run dangerous command")
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()

		cmd := exec.CommandContext(timeoutCtx, "bash", "-lc", command)
		cmd.Dir = safety.Workdir
		outputBytes, runErr := cmd.CombinedOutput()
		output := string(outputBytes)

		if timeoutCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("Command timed out after %ds", timeoutSec)
		}

		if runErr != nil && output == "" {
			output = runErr.Error()
		}
		if output == "" {
			output = "[no output]"
		}

		return safety.Truncate(output), nil
	}
}
