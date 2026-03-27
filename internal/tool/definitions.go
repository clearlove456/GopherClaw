package tool

import "github.com/shencheng/GopherClaw/internal/model"

func Schemas() []model.ToolSchema {
	return []model.ToolSchema{
		{
			Type: "function",
			Function: model.ToolDefinition{
				Name: "bash",
				Description: "Run a shell command and return its output. " +
					"Use for system commands, git, package managers, etc.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "The shell command to execute.",
						},
						"timeout": map[string]any{
							"type":        "integer",
							"description": "Timeout in seconds. Default 30.",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		{
			Type: "function",
			Function: model.ToolDefinition{
				Name:        "read_file",
				Description: "Read the contents of a file.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_path": map[string]any{
							"type":        "string",
							"description": "Path to the file (relative to working directory).",
						},
					},
					"required": []string{"file_path"},
				},
			},
		},
		{
			Type: "function",
			Function: model.ToolDefinition{
				Name: "write_file",
				Description: "Write content to a file. Creates parent directories if needed. " +
					"Overwrites existing content.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_path": map[string]any{
							"type":        "string",
							"description": "Path to the file (relative to working directory).",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "The content to write.",
						},
					},
					"required": []string{"file_path", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: model.ToolDefinition{
				Name: "edit_file",
				Description: "Replace an exact string in a file with a new string. " +
					"The old_string must appear exactly once in the file. " +
					"Always read the file first to get the exact text to replace.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_path": map[string]any{
							"type":        "string",
							"description": "Path to the file (relative to working directory).",
						},
						"old_string": map[string]any{
							"type":        "string",
							"description": "The exact text to find and replace. Must be unique.",
						},
						"new_string": map[string]any{
							"type":        "string",
							"description": "The replacement text.",
						},
					},
					"required": []string{"file_path", "old_string", "new_string"},
				},
			},
		},
	}
}
