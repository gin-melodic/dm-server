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

1. **Start PostgreSQL:**
   ```bash
   # Container deployment is managed by the parent repository's unified Compose config.
   cd ..
   docker compose up -d postgres
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

## 🚢 VPS Modular Releases

The parent repository provides formal module-level release scripts for an already initialized VPS deployment. Use this path after the first full offline bundle has been deployed with the root `scripts/build-vps-bundle.sh` and remote `deploy.sh`.

### Prerequisites

The local machine needs Docker Buildx, `ssh`, `rsync`, `gzip`, and `git`. The remote Debian 12 host requires Docker Compose v2 and `rsync`:

```bash
ssh -p <ssh-port> <user>@<vps-host>
sudo apt update
sudo apt install -y rsync
rsync --version
```

The remote deployment directory defaults to `$HOME/dm-proj-vps`, or can be overridden with `VPS_DEPLOY_DIR`. The VPS `.env` file is updated by the scripts to record the current module image version, ensuring subsequent restarts use the same version.

### Release `dm-server` Only

Use this for Go API logic updates. The script builds, compresses, and uploads only the versioned `dm-server` image and recreates just the `dm-server` container; it does not upload PostgreSQL or `dm-knowledge-service` images, nor restart the database or knowledge service.

```bash
cd ..
VPS_HOST=203.0.113.10 \
VPS_USER=deploy \
VPS_SSH_PORT=2222 \
VPS_DEPLOY_DIR=/home/deploy/dm-proj-vps \
DM_SERVER_VERSION=v1.2.0 \
./scripts/release-dm-server-vps.sh
```

Common variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `VPS_HOST` | required | VPS IP or domain |
| `VPS_USER` | required | SSH user |
| `VPS_SSH_PORT` | `22` | SSH port |
| `VPS_DEPLOY_DIR` | `dm-proj-vps` | Remote deployment directory |
| `PLATFORM` | `linux/amd64` | Use `linux/arm64` for ARM VPS |
| `DM_SERVER_VERSION` | UTC timestamp + git revision | Docker image tag |
| `HEALTH_TIMEOUT` | `90` | Health wait seconds |

After a successful release, the remote `.env` updates `DM_SERVER_IMAGE=dm-server:<version>`, then runs:

```bash
docker compose up -d --no-deps --force-recreate dm-server
```

If the new container fails the health check, the script rolls back `DM_SERVER_IMAGE` to the previous tag and restarts the old container.

### Release `dm-knowledge-service` Only

Use this for Python RAG service logic updates or knowledge-base content/index changes. The script builds, compresses, and uploads only the versioned `dm-knowledge-service` image and recreates just the `dm-knowledge-service` container; it does not upload PostgreSQL or `dm-server` images.

For logic-only updates (do not refresh the knowledge-base volume):

```bash
cd ..
VPS_HOST=203.0.113.10 \
VPS_USER=deploy \
VPS_SSH_PORT=2222 \
VPS_DEPLOY_DIR=/home/deploy/dm-proj-vps \
DM_KNOWLEDGE_SERVICE_VERSION=v1.2.0 \
./scripts/release-dm-knowledge-service-vps.sh
```

If this release includes knowledge-base text, pre-built index, or embedding configuration changes in `dm-knowledge-service/data/knowledge_bases`, set the remote `knowledge_data` volume to be re-seeded from the new image:

```bash
cd ..
VPS_HOST=203.0.113.10 \
VPS_USER=deploy \
VPS_SSH_PORT=2222 \
VPS_DEPLOY_DIR=/home/deploy/dm-proj-vps \
DM_KNOWLEDGE_SERVICE_VERSION=v1.2.0 \
REFRESH_KNOWLEDGE_DATA=1 \
./scripts/release-dm-knowledge-service-vps.sh
```

`REFRESH_KNOWLEDGE_DATA=1` deletes and recreates only the `dm-proj_knowledge_data` volume; it does not touch the PostgreSQL data or log volumes. For regular logic releases, keep `REFRESH_KNOWLEDGE_DATA=0` to avoid overwriting existing knowledge-base data on the remote host.

Common variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DM_KNOWLEDGE_SERVICE_VERSION` | UTC timestamp + git revision | Docker image tag |
| `REFRESH_KNOWLEDGE_DATA` | `0` | Set `1` only for knowledge/index refresh |
| `HEALTH_TIMEOUT` | `180` | RAG startup and health wait seconds |
| `EMBEDDING_PROVIDER` | `local` | Build-time fallback provider |
| `INDEX_EMBEDDING_PROVIDER` | from `.env`/Dockerfile | Provider used to prebuild the offline index |

After a successful release, the remote `.env` updates `DM_KNOWLEDGE_SERVICE_IMAGE=dm-knowledge-service:<version>`, then runs:

```bash
docker compose up -d --no-deps --force-recreate dm-knowledge-service
```

If the health check fails, the script rolls back to the previous `DM_KNOWLEDGE_SERVICE_IMAGE` and restarts the old container. When `knowledge_data` was refreshed during the release, rollback restores only the container image — it does not automatically restore the previous knowledge volume. Before releasing knowledge-base or index changes in production, back up the volume on the VPS first.

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
