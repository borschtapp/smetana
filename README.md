# Smetana* 🇺🇦

<p align="center">
    <a href="https://github.com/borschtapp/smetana/tags"><img src="https://img.shields.io/github/v/tag/borschtapp/smetana" alt="semver tag" title="semver tag"/></a>
    <a href="https://goreportcard.com/report/github.com/borschtapp/smetana"><img src="https://goreportcard.com/badge/github.com/borschtapp/smetana" alt="go report card" title="go report card"/></a>
    <a href="https://github.com/borschtapp/smetana/blob/main/LICENSE"><img src="https://img.shields.io/github/license/borschtapp/smetana" alt="license" title="license"/></a>
</p>

Cookbooks and meal planning backend for [Borscht App](https://github.com/borschtapp/). Scrapes, stores, processes, and serves recipes via a REST API.

\* _Smetana_ is a Ukrainian word for _sour cream_. No Borscht is complete without it.

---

## Features

- **Recipe management** — create, search, and update recipes with structured ingredients, nutrition, and instructions.
- **Recipe import** — scrape any recipe URL using [krip](https://github.com/borschtapp/krip).
- **Feeds & discovery** — subscribe to RSS/Atom feeds with scheduled background ingestion.
- **Households** — shared workspaces with invite codes, member management, and ownership transfer.
- **Collections & Meal plans** — custom recipe collections and household calendar scheduling.
- **Shopping lists** — grocery aggregation with ingredient merging, unit conversion, and item check-offs.
- **Prices & costs** — historical grocery prices and recipe ingredient cost estimates.
- **Authentication** — JWT sessions with refresh revocation, password reset via SMTP, and optional OIDC SSO.
- **Image storage** — local filesystem or S3-compatible object storage (AWS S3, MinIO, Cloudflare R2).

---

## Quickstart

### Prerequisites
- Go 1.26+ and a C compiler (gcc/musl) for SQLite CGO support.

### Run locally
```bash
cp .env.example .env   # configure JWT_SECRET_KEY
go run main.go
```
The server starts on <http://localhost:3000>. Swagger UI is available at `/`.

### Run with Docker Compose
```bash
docker compose up -d
```
Mounts `./data` to `/app/data` and publishes port `3000` on all host interfaces. For PostgreSQL, uncomment the `postgres` service and `depends_on` in `compose.yaml`, then set `DB_TYPE=postgres`, `DB_HOST=postgres`, `DB_USER`, `DB_PASSWORD` and `DB_NAME` in `.env` (both containers read them).

---

## Development

```bash
# Build binary
go build -o smetana .

# Run test suite
go test ./...

# Regenerate Swagger docs (requires swag CLI)
swag init
```

---

## Configuration

All configuration is managed through environment variables or `.env`. Only `JWT_SECRET_KEY` is required.

### Server & Security
| Variable                     | Default                 | Description                                     |
|------------------------------|-------------------------|-------------------------------------------------|
| `BASE_URL`                   | —                       | Public base URL for links and uploads           |
| `SERVER_HOST`                | `""`                    | Bind host (default all interfaces)              |
| `SERVER_PORT`                | `3000`                  | HTTP listen port                                |
| `SERVER_READ_TIMEOUT`        | `15`                    | Request read timeout in seconds                 |
| `SERVER_WRITE_TIMEOUT`       | `30`                    | Response write timeout in seconds               |
| `SERVER_IDLE_TIMEOUT`        | `120`                   | Keep-alive idle timeout in seconds              |
| `JWT_SECRET_KEY`             | —                       | **Required**. Secret key for JWT signing        |
| `JWT_SECRET_EXPIRE_MINUTES`  | `60`                    | Access token expiration in minutes              |
| `JWT_REFRESH_EXPIRE_MINUTES` | `10080`                 | Refresh token expiration in minutes (7d)        |

### Database & Storage
| Variable                          | Default             | Description                                                |
|-----------------------------------|---------------------|------------------------------------------------------------|
| `DB_TYPE`                         | `sqlite`            | `sqlite`, `postgres`, or `mysql`                           |
| `DB_NAME`                         | `./data/borscht.db` | SQLite path or database name                               |
| `DB_HOST` / `DB_PORT`             | `localhost` / —     | DB host and port (defaults: MySQL `3306`, Postgres `5432`) |
| `DB_USER` / `DB_PASSWORD`         | —                   | Database credentials                                       |
| `DB_SSLMODE`                      | `disable`           | Postgres SSL mode (`disable`, `require`, `verify-full`)    |
| `STORAGE_ROOT`                    | `./data/uploads`    | Local media uploads directory                              |
| `S3_BUCKET`                       | —                   | S3 bucket name (enables S3 storage)                        |
| `S3_ENDPOINT`                     | —                   | S3 endpoint URL (MinIO, R2, AWS)                           |
| `S3_REGION`                       | `us-east-1`         | S3 region                                                  |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | —                   | S3 credentials                                             |
| `S3_TIMEOUT`                      | `10`                | S3 request timeout in seconds                              |

### Jobs, Scraping & Middleware
| Variable                          | Default              | Description                                              |
|-----------------------------------|----------------------|----------------------------------------------------------|
| `FETCH_INTERVAL`                  | `24h`                | Feed sync interval (e.g., `24h`, `1h`)                   |
| `FETCH_CONCURRENCY`               | `5`                  | Max parallel fetches per feed sync run or recipe import  |
| `FEED_SYNC_CONCURRENCY`           | `2`                  | Concurrency limit for subscription sync                  |
| `DOWNLOAD_TIMEOUT`                | `30s`                | Image download timeout                                   |
| `SCRAPE_TIMEOUT`                  | `30s`                | Recipe page scrape timeout (504 when exceeded)           |
| `USER_AGENT`                      | Browser UA           | User-Agent header for image downloads                    |
| `ENABLE_LIMITER`                  | `false`              | Enable global rate limiting (auth routes always limited) |
| `ENABLE_COMPRESS`                 | `false`              | Enable gzip/brotli response compression                  |
| `LOG_LEVEL` / `LOG_TARGET`        | `info` / `console`   | Logging severity and target (`console`, `file`, `both`)  |
| `LOG_FILE_PATH`                   | `./data/smetana.log` | File path when `LOG_TARGET` includes `file`              |
| `LOG_REQUESTS` / `LOG_DB_QUERIES` | `false` / `false`    | Log HTTP requests and SQL queries                        |

### Email (SMTP) & OIDC
| Variable                                | Default   | Description                                                      |
|-----------------------------------------|-----------|------------------------------------------------------------------|
| `SMTP_HOST` / `SMTP_PORT`               | — / `587` | SMTP server and port (enables password reset/invites)            |
| `SMTP_USER` / `SMTP_PASSWORD`           | —         | SMTP credentials                                                 |
| `SMTP_FROM`                             | —         | Outgoing email sender address                                    |
| `OIDC_PROVIDER`                         | —         | OIDC discovery URL (e.g. `https://accounts.google.com`)          |
| `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` | —         | OAuth2 client credentials                                        |
| `OIDC_REDIRECT_URL`                     | derived   | Callback URL, defaults to `<BASE_URL>/api/v1/auth/oidc/callback` |

---

## API Overview

All routes are prefixed with `/api/v1`. Protected endpoints require `Authorization: Bearer <token>`.

| Group          | Prefix           | Auth     | Description                                                     |
|----------------|------------------|----------|-----------------------------------------------------------------|
| Auth           | `/auth`          | Public   | Register, login, refresh, logout, password reset, OIDC callback |
| Users          | `/users`         | Required | User profile, password update, account deletion                 |
| Households     | `/households`    | Required | Household management, members, invites                          |
| Collections    | `/collections`   | Required | Recipe collection CRUD                                          |
| Meal plan      | `/mealplan`      | Required | Calendar meal planning                                          |
| Shopping lists | `/shoppinglists` | Required | Lists and batch items with auto-conversions                     |
| Food & prices  | `/food`          | Required | Shared foods, price tracking, unit conversions                  |
| Units          | `/units`         | Required | Measurement units and conversion factors                        |
| Recipes        | `/recipes`       | Required | Recipe CRUD, search, scraping, favorites                        |
| Import         | `/import`        | Required | Import recipes from a URL                                       |
| Authors        | `/authors`       | Required | Recipe authors                                                  |
| Equipment      | `/equipment`     | Required | Kitchen equipment                                               |
| Feeds          | `/feeds`         | Required | RSS/Atom feed subscriptions                                     |
| Publishers     | `/publishers`    | Required | Recipe publishers                                               |
| Taxonomies     | `/taxonomies`    | Required | Tags, cuisines, meal types                                      |
| Uploads        | `/uploads`       | Required | Image upload endpoint                                           |

Interactive API documentation is served at `/` (Swagger UI).

---

## CLI Flags

| Flag           | Description                                   |
|----------------|-----------------------------------------------|
| `--no-migrate` | Skip automatic database migrations on startup |

---

## License

GNU General Public License Version 3 (GPL-3.0) — see [LICENSE](LICENSE).
