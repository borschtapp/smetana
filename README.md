# Smetana* 🇺🇦

<p style="text-align: center;">
    <a href="https://github.com/borschtapp/smetana/tags"><img src="https://img.shields.io/github/v/tag/borschtapp/smetana" alt="semver tag" title="semver tag"/></a>
    <a href="https://goreportcard.com/report/github.com/borschtapp/smetana"><img src="https://goreportcard.com/badge/github.com/borschtapp/smetana" alt="go report card" title="go report card"/></a>
    <a href="https://github.com/borschtapp/smetana/blob/main/LICENSE"><img src="https://img.shields.io/github/license/borschtapp/smetana" alt="license" title="license"/></a>
</p>

Cookbooks management backend for [Borscht App](https://borscht.app). Scrapes, stores, processes, and serves recipes via a REST API.

* _Smetana_ is a Ukrainian word for _sour cream_. No Borscht is complete without it.

---

## Features

- **Recipe management** — create, search, and update recipes with structured ingredients and step-by-step instructions
- **Recipe import** — scrape any recipe URL using [krip](https://github.com/borschtapp/krip); images are downloaded and stored locally
- **Feeds** — subscribe to RSS/Atom feeds; a background job fetches new recipes on a configurable interval
- **Households** — shared workspaces; invite new members via a short code, transfer ownership, remove members
- **Collections** — named recipe lists per household (bookmarks, favorites, etc.)
- **Meal plans** — schedule recipes across dates per household
- **Shopping lists** — household shopping lists with per-item management
- **Authentication** — JWT-based sessions with refresh tokens; password reset via email; optional OpenID Connect (OIDC) SSO via any compliant provider
- **Image storage** — local filesystem (default) or S3-compatible object storage
- **API docs** — Swagger UI served at the root (`/`)

---

## Tech Stack

| Layer           | Library                                                                                                               |
|-----------------|-----------------------------------------------------------------------------------------------------------------------|
| HTTP framework  | [Fiber v3](https://github.com/gofiber/fiber)                                                                          |
| ORM             | [GORM](https://gorm.io)                                                                                               |
| Database        | SQLite (default), MySQL, PostgreSQL                                                                                   |
| Auth            | JWT ([golang-jwt/jwt](https://github.com/golang-jwt/jwt)), OIDC ([coreos/go-oidc](https://github.com/coreos/go-oidc)) |
| Recipe scraping | [borschtapp/krip](https://github.com/borschtapp/krip)                                                                 |
| Recipe parsing  | [borschtapp/kapusta](https://github.com/borschtapp/kapusta)                                                           |
| Scheduler       | [gocron v2](https://github.com/go-co-op/gocron)                                                                       |
| Object storage  | AWS S3 SDK v2 / [gofiber/storage/s3](https://github.com/gofiber/storage)                                              |
| API docs        | [swaggo/swag](https://github.com/swaggo/swag)                                                                         |

---

## Getting Started

### Prerequisites

- Go 1.26+

### Run locally

```bash
go run main.go
```

The server starts on <http://localhost:3000>. Swagger UI is available at `/`.

### Environment variables

Copy `.env.example` to `.env` and adjust as needed. All variables are optional; defaults are shown below.

#### Server

| Variable      | Default                 | Description                                                    |
|---------------|-------------------------|----------------------------------------------------------------|
| `SERVER_HOST` | `` (all interfaces)     | Bind address                                                   |
| `SERVER_PORT` | `3000`                  | Listen port                                                    |
| `BASE_URL`    | `https://{SERVER_HOST}` | Public base URL — used in image URLs and password reset emails |

#### Database

| Variable             | Default             | Description                                        |
|----------------------|---------------------|----------------------------------------------------|
| `DB_TYPE`            | `sqlite`            | `sqlite`, `mysql`, or `postgres`                   |
| `DB_HOST`            | `localhost`         | Database host (MySQL/Postgres only)                |
| `DB_PORT`            | `3306`              | Database port (MySQL/Postgres only)                |
| `DB_NAME`            | `./data/borscht.db` | Database name or file path                         |
| `DB_USER`            | —                   | Database user (MySQL/Postgres only)                |
| `DB_PASSWORD`        | —                   | Database password (MySQL/Postgres only)            |
| `DB_SSLMODE`         | `disable`           | SSL mode for Postgres (`disable`, `require`, etc.) |
| `GORM_ENABLE_LOGGER` | `false`             | Enable GORM SQL query logging                      |

#### Storage

By default files are stored under `./data/uploads` and served at `/uploads`.

To use S3-compatible storage set:

| Variable        | Description                                             |
|-----------------|---------------------------------------------------------|
| `S3_BUCKET`     | Bucket name (enables S3 mode)                           |
| `S3_ENDPOINT`   | Endpoint URL (e.g. `s3.amazonaws.com` or a custom host) |
| `S3_REGION`     | Region (default `us-east-1`)                            |
| `S3_ACCESS_KEY` | Access key ID                                           |
| `S3_SECRET_KEY` | Secret access key                                       |

#### Authentication

| Variable                      | Default  | Description                              |
|-------------------------------|----------|------------------------------------------|
| `JWT_SECRET_KEY`              | —        | Secret key used to sign JWT tokens       |
| `JWT_SECRET_EXPIRE_MINUTES`   | `60`     | Access token lifetime in minutes         |
| `JWT_REFRESH_EXPIRE_MINUTES`  | `10080`  | Refresh token lifetime in minutes (7d)   |

#### Email / Password reset (optional)

Required for `POST /auth/forgot-password`. Password reset is disabled if `SMTP_HOST` is not set.

| Variable        | Default | Description                                      |
|-----------------|---------|--------------------------------------------------|
| `SMTP_HOST`     | —       | SMTP server hostname (enables email support)     |
| `SMTP_PORT`     | `587`   | SMTP port                                        |
| `SMTP_USER`     | —       | SMTP username                                    |
| `SMTP_PASSWORD` | —       | SMTP password                                    |
| `SMTP_FROM`     | —       | Sender address (e.g. `noreply@example.com`)      |

#### OIDC / SSO (optional)

| Variable             | Description                                                 |
|----------------------|-------------------------------------------------------------|
| `OIDC_PROVIDER`      | Provider discovery URL (e.g. `https://accounts.google.com`) |
| `OIDC_CLIENT_ID`     | OAuth2 client ID                                            |
| `OIDC_CLIENT_SECRET` | OAuth2 client secret                                        |
| `OIDC_REDIRECT_URL`  | Callback URL registered with the provider                   |

OIDC support is disabled if any of the required variables are missing.

#### Background jobs

| Variable         | Default | Description                                          |
|------------------|---------|------------------------------------------------------|
| `FETCH_INTERVAL` | `24h`   | How often to fetch new recipes from subscribed feeds |

#### Middleware toggles

| Variable          | Default | Description                                                          |
|-------------------|---------|----------------------------------------------------------------------|
| `ENABLE_LIMITER`  | `false` | Enable rate limiting on all routes (auth routes always rate-limited) |
| `ENABLE_COMPRESS` | `false` | Enable gzip/brotli response compression                              |
| `ENABLE_LOGGER`   | `true`  | Enable request logging                                               |

---

## API Overview

All endpoints are prefixed with `/api/v1`. Protected endpoints require a `Authorization: Bearer <token>` header.

| Group          | Prefix           | Auth     | Description                                                       |
|----------------|------------------|----------|-------------------------------------------------------------------|
| Auth           | `/auth`          | Public   | Register, login, refresh, logout, password reset, OIDC flow       |
| Users          | `/users`         | Required | Get, update (incl. password change), delete user profile          |
| Households     | `/households`    | Required | Household details, member management, invite codes                |
| Collections    | `/collections`   | Required | Recipe collections CRUD + recipe membership                       |
| Meal plan      | `/mealplan`      | Required | Per-household meal schedule                                       |
| Shopping lists | `/shoppinglists` | Required | Shopping lists and items                                          |
| Recipes        | `/recipes`       | Required | Recipe CRUD, search, import, ingredients, instructions, favorites |
| Feeds          | `/feeds`         | Required | RSS/Atom subscriptions and aggregated stream                      |
| Publishers     | `/publishers`    | Required | Publisher lookup                                                  |
| Taxonomies     | `/taxonomies`    | Required | Tag/category lookup                                               |
| Uploads        | `/uploads`       | Required | Direct image upload                                               |

Full interactive documentation is served at `/` (Swagger UI).

---

## Project Structure

```
domain/          # Domain types and repository/service interfaces
internal/
  configs/       # Fiber, GORM, storage, JWT, email configuration
  database/      # DB connection and GORM auto-migrations
  handlers/api/  # Fiber HTTP handlers
  jobs/          # Background job implementations
  middlewares/   # JWT auth middleware
  repositories/  # GORM repository implementations
  routes/        # Dependency wiring and route registration
  scheduler/     # gocron scheduler wrapper
  sentinels/     # Shared error and HTTP sentinel values
  services/      # Business logic layer
  storage/       # FileStorage interface + local and S3 backends
  tokens/        # JWT generation and parsing
  types/         # Shared types (pagination, search, response wrappers)
  utils/         # Environment helpers, password hashing, etc.
docs/            # Swagger generated docs
main.go          # Entry point
```

---

## CLI Flags

| Flag           | Description                                   |
|----------------|-----------------------------------------------|
| `--no-migrate` | Skip automatic database migrations on startup |

---

## License

GNU GPL Version 3 — see [LICENSE](LICENSE).
