# 🌙 Dream Master (dm-server)

An AI-powered dream analysis service built for WeChat Mini Program. Describe your dreams, get psychological interpretations powered by large language models — all backed by insights from Freud, Jung and more. 🔮

## ✨ Features

- 💬 **Real-time Analysis** — WebSocket streaming delivers LLM-powered dream interpretations as they're generated
- 🧠 **Knowledge-Enhanced Prompts** — Enriched with relevant excerpts from classic works like *The Interpretation of Dreams*
- 🔄 **Multiple AI Providers** — Pluggable support for Ollama, OpenRouter, Alibaba Qwen, Tencent Hunyuan, and LMStudio
- 👤 **WeChat Authentication** — Seamless Mini Program login with JWT-based session management
- 📜 **Dream History** — Browse, search, and manage your past dream records
- ⚡ **Rate Limiting** — Built-in concurrency control and per-user request limits

## 🛠 Tech Stack

Go 1.25 · GoFrame v2 · PostgreSQL 18.4 · WebSocket · JWT

## 🚀 Quick Start

**Prerequisites:** Go 1.25+, PostgreSQL 18.4

1. **Start PostgreSQL with Docker Compose:**
   ```bash
   docker-compose up -d
   ```

2. **Configure the application:**
   ```bash
   cp manifest/config/config.yaml.example manifest/config/config.yaml
   # Edit config.yaml with your credentials
   ```

3. **Run the server:**
   ```bash
   gf run main.go
   # or
   go run main.go
   ```

Server starts on `http://localhost:8001` by default. 🎉

## 📡 Endpoints

| Endpoint | Description |
|----------|-------------|
| `/api/v1/wechat/auth` | WeChat Mini Program login |
| `/api/v1/user/info` | Get or update user profile |
| `/api/v1/dream/list` | Paginated dream history |
| `/api/v1/dream/delete` | Soft-delete a dream record |
| `/api/v1/dream/analyze/result` | Retrieve analysis result |
| `/api/v1/chat/ws` | WebSocket for real-time dream analysis |

Dev tools are available at `/dev/api-test` and Swagger UI at `/swagger`. 🔧

## 📁 Project Structure

```
internal/
├── controller/    # HTTP handlers
├── logic/         # Business logic (auth, dream analysis, history)
├── middleware/    # JWT auth, CORS, response formatting
├── model/         # Data structures and entities
├── router/        # Route registration
├── service/       # Service interface definitions
└── utility/       # Concurrency limiter, rate limiter
```

## ⚙️ Configuration

Environment variables can override config values at runtime: `PORT`, `DB_USER`, `DB_PASSWORD`, `DB_HOST`, `DB_PORT`, `DB_NAME`. See `manifest/config/config.yaml.example` for the full configuration reference.

For more on GoFrame, visit https://goframe.org/quick
