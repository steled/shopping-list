# Project Guidelines — Shopping List

## Overview

Lightweight single-user shopping list web application.

- **Backend**: Go 1.22, stdlib `net/http` (Go 1.22+ method routing), `html/template`, `database/sql`
- **Auth**: HMAC-SHA256 signed session cookie, bcrypt password validation, single user
- **Database**: SQLite via `modernc.org/sqlite` (CGO-free pure Go)
- **Frontend**: Vanilla JS, Vanilla CSS (CSS vars, dark/light mode), SortableJS CDN
- **Docker**: `golang:1.22-alpine` builder → `alpine:3.21` final (~15MB), non-root user `appuser` (uid 1001), multi-arch (amd64 + arm64)
- **Helm chart**: `helm/shopping-list/`, API v2, Gateway API HTTPRoute (default) or Ingress, SQLite on PVC
- **Tests**: `go test -v -race ./...`
- **Lint**: `golangci-lint run`

## Repository

- **GHE org**: `datagroup.ghe.com/DGOPS`
- **Remote**: `https://datagroup.ghe.com/DGOPS/cloud-ops.k8s.shopping-list.git`
- Always use `GH_HOST=datagroup.ghe.com` with `gh` CLI commands
- **Branch protection on `main`**: all changes go through PRs — never push directly to `main`

## Registries

| Artifact  | Registry                                                               |
|-----------|------------------------------------------------------------------------|
| Docker    | `containers.datagroup.ghe.com/dgops/shopping-list`                     |
| Docker Hub| `dgops/shopping-list`                                                  |
| Helm OCI  | `oci://containers.datagroup.ghe.com/dgops/charts/shopping-list`        |

## Commit Convention (Conventional Commits, commitlint enforced)

- Format: `type(scope): subject` — **subject must be fully lowercase**
- Valid types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`, `build`, `revert`
- `feat:` → minor bump, `fix:`/`perf:` → patch, `BREAKING CHANGE` / `type!:` → major
- Commitlint rule: `subject-case: lower-case` — fails on any uppercase in subject

## Project Structure

```
main.go                     # HTTP server, routes, embed directives
internal/
  auth/auth.go              # HMAC session cookie, bcrypt login
  database/database.go      # SQLite CRUD + reorder
  handlers/handlers.go      # HTTP handlers
templates/                  # Embedded HTML templates (Go html/template)
  base.html                 # Layout + dark mode toggle
  login.html                # Login form
  index.html                # Shopping list + SortableJS
static/                     # Embedded static assets
  style.css                 # CSS vars light/dark theme
docker/
  Dockerfile                # Multi-stage: golang:1.24-alpine → alpine:3.21
helm/shopping-list/         # Helm chart
```

## Environment Variables

| Variable            | Description                              | Default              |
|---------------------|------------------------------------------|----------------------|
| `APP_USERNAME`      | Login username                           | `admin`              |
| `APP_PASSWORD`      | Login password (bcrypt-compared)         | **required**         |
| `APP_SESSION_SECRET`| HMAC key for session cookie signing      | **required**         |
| `DATABASE_PATH`     | SQLite file path                         | `/data/shopping.db`  |
| `APP_ADDR`          | Listen address                           | `:8080`              |

## API

| Method   | Path                     | Description                        |
|----------|--------------------------|------------------------------------|
| `GET`    | `/healthz`               | Health check                       |
| `GET`    | `/login`                 | Login form                         |
| `POST`   | `/login`                 | Authenticate                       |
| `GET`    | `/logout`                | Clear session, redirect to /login  |
| `GET`    | `/list`                  | Shopping list page (auth required) |
| `GET`    | `/api/items`             | List items (JSON)                  |
| `POST`   | `/api/items`             | Create item                        |
| `PUT`    | `/api/items/{id}`        | Update item (name/qty/checked)     |
| `DELETE` | `/api/items/{id}`        | Delete item                        |
| `PATCH`  | `/api/items/reorder`     | Reorder items `{ids:[...]}`        |

## Workflows

| File                                      | Trigger                                                    |
|-------------------------------------------|------------------------------------------------------------|
| `.github/workflows/test.yaml`             | PR / push to main on relevant paths                        |
| `.github/workflows/release.yaml`          | `push: tags: v*` or `workflow_dispatch`                    |
| `.github/workflows/auto-release.yaml`     | push to main → conventional commits → auto-tag             |
| `.github/workflows/commitlint.yaml`       | all PRs                                                    |
| `.github/workflows/renovate.yaml`         | schedule Monday midnight (reusable)                        |
| `.github/workflows/renovate-go-tidy.yml`  | Renovate PRs touching `go.mod`/`go.sum` → `go mod tidy`   |

### Key workflow constraints

- `GITHUB_TOKEN` has **no `workflows` permission** → exclude `.github/workflows/` from auto-commits
- Changelog uses **tab** (`%x09`) as git log separator
