package builtin

import "github.com/shencheng/GopherClaw/internal/tool"

func RegisterAll(reg *tool.Registry, safety *tool.Safety) {
	reg.Register("bash", Bash(safety))
	reg.Register("read_file", ReadFile(safety))
	reg.Register("write_file", WriteFile(safety))
	reg.Register("edit_file", EditFile(safety))
}
