## GopherClaw

Section 01 (Go): Agent loop with OpenAI-compatible API.

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
