## GopherClaw

Section 02 (Go): Agent loop + tool use with OpenAI-compatible API.

### Run

1. Create `.env` from `.env.example` and fill your API key.
2. Start:

```bash
go run ./cmd/claw
```

### Environment Variables

- `OPENAI_API_KEY` (required)
- `OPENAI_BASE_URL` (optional, default: `https://api.openai.com/v1`)
- `MODEL_ID` (optional, default: `gpt-4o-mini`)
- `MAX_TOKENS` (optional, default: `8096`)
- `SYSTEM_PROMPT` (optional)

### Built-in Tools

- `bash`
- `read_file`
- `write_file`
- `edit_file`
