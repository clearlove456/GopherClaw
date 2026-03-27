package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultMaxToolOutput = 50000
)

type Safety struct {
	Workdir       string
	MaxToolOutput int
}

func NewSafety(workdir string, maxToolOutput int) (*Safety, error) {
	if strings.TrimSpace(workdir) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		workdir = wd
	}

	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("resolve workdir: %w", err)
	}

	limit := maxToolOutput
	if limit <= 0 {
		limit = DefaultMaxToolOutput
	}

	return &Safety{
		Workdir:       absWorkdir,
		MaxToolOutput: limit,
	}, nil
}

func (s *Safety) SafePath(raw string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("safety is not initialized")
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("empty file path")
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("Path traversal blocked: absolute path is not allowed")
	}

	target := filepath.Clean(filepath.Join(s.Workdir, trimmed))
	if target != s.Workdir && !strings.HasPrefix(target, s.Workdir+string(os.PathSeparator)) {
		return "", fmt.Errorf("Path traversal blocked: %s resolves outside WORKDIR", raw)
	}

	return target, nil
}

func (s *Safety) Truncate(text string) string {
	if s == nil || s.MaxToolOutput <= 0 || len(text) <= s.MaxToolOutput {
		return text
	}
	return text[:s.MaxToolOutput] + fmt.Sprintf("\n... [truncated, %d total chars]", len(text))
}

func IsDangerousCommand(command string) bool {
	dangerous := []string{
		"rm -rf /",
		"mkfs",
		"> /dev/sd",
		"dd if=",
	}
	for _, pattern := range dangerous {
		if strings.Contains(command, pattern) {
			return true
		}
	}
	return false
}
