# 🛒 Shopping List

[![Test](https://github.com/steled/shopping-list/actions/workflows/test.yaml/badge.svg)](https://github.com/steled/shopping-list/actions/workflows/test.yaml)
[![Latest Tag](https://img.shields.io/github/v/tag/steled/shopping-list?label=release)](https://github.com/steled/shopping-list/tags)
[![Go Version](https://img.shields.io/badge/go-1.22-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

A lightweight, self-hosted shopping list web application built with Go and SQLite.

## Table of Contents

- [Features](#features)
- [Tech Stack](#tech-stack)
- [Local Development](#local-development)
- [Environment Variables](#environment-variables)
- [Docker](#docker)
- [Kubernetes (Helm)](#kubernetes-helm)
- [API Reference](#api-reference)
- [CI/CD](#cicd)
- [Versioning](#versioning)
- [License](#license)

## Features

- **Single-user authentication** — bcrypt password hashing, HMAC-signed session cookies
- **Shopping list management** — add, edit, delete and check off items with quantities
- **Drag & drop reordering** — reorder items via SortableJS (bundled, no CDN)
- **Filter view** — hide already-checked items with one click
- **Dark mode** — automatic via `prefers-color-scheme`, toggle persisted in `localStorage`
- **Accessible UI** — WCAG-compliant touch targets (≥ 44 px), visible `focus-visible` indicators on all interactive elements, sufficient contrast ratios in both themes
- **Zero dependencies at runtime** — single static binary with embedded templates and assets
- **SQLite persistence** — no external database required
- **Security hardened** — strict CSP (`default-src 'self'`), `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`, enforced `Secure` cookie flag behind reverse proxies

## Tech Stack

| Layer      | Technology                                |
|------------|-------------------------------------------|
| Backend    | Go 1.22, `net/http` stdlib                |
| Database   | SQLite via `modernc.org/sqlite` (pure Go, CGO-free) |
| Frontend   | Vanilla JS + CSS, [SortableJS](https://sortablejs.github.io/Sortable/) (bundled in `static/`) |
| Container  | Multi-stage Docker build (~15 MB image)   |
| Deployment | Kubernetes Helm chart (Gateway API or Ingress) |

## Local Development

**Prerequisites:** Go 1.22+

```bash
# Clone
git clone https://github.com/steled/shopping-list.git
cd shopping-list

# Install dependencies
go mod download

# Run (password and session secret are required)
APP_PASSWORD=secret APP_SESSION_SECRET=$(openssl rand -hex 32) go run .
```

The app listens on `http://localhost:8080`. Log in with username `admin` and the password you set.

### Running tests

```bash
go test ./...
```

## Environment Variables

| Variable              | Default             | Description                                              |
|-----------------------|---------------------|----------------------------------------------------------|
| `APP_USERNAME`        | `admin`             | Login username                                           |
| `APP_PASSWORD`        | *(required)*        | Login password (plain text; hashed with bcrypt at startup) |
| `APP_SESSION_SECRET`  | *(required)*        | HMAC secret for session cookies (**min. 32 characters** — use `openssl rand -hex 32`) |
| `APP_SECURE_COOKIES`  | `false`             | Set to `true` when running behind a TLS-terminating reverse proxy (Ingress/Gateway) to enforce the `Secure` flag on session cookies. Defaults to `true` in the Helm chart. |
| `DATABASE_PATH`       | `/data/shopping.db` | Path to the SQLite database file                        |
| `APP_ADDR`            | `:8080`             | HTTP listen address                                      |

## Docker

```bash
# Build
docker build -f docker/Dockerfile -t shopping-list .

# Run
docker run -p 8080:8080 \
  -e APP_PASSWORD=secret \
  -e APP_SESSION_SECRET=$(openssl rand -hex 32) \
  -v shopping-data:/data \
  shopping-list
```

Or pull the pre-built image from GHCR:

```bash
docker pull ghcr.io/steled/shopping-list:latest
```

## Kubernetes (Helm)

```bash
helm install shopping-list oci://ghcr.io/steled/charts/shopping-list \
  --version 0.3.0 \
  --set auth.password=secret \
  --set auth.sessionSecret=$(openssl rand -hex 32) \
  --set networking.gateway.hostname=shopping-list.example.com
```

Or using an existing Secret:

```bash
# Create secret first
kubectl create secret generic shopping-list \
  --from-literal=username=admin \
  --from-literal=password=secret \
  --from-literal=session-secret=$(openssl rand -hex 32)

helm install shopping-list oci://ghcr.io/steled/charts/shopping-list \
  --version 0.3.0 \
  --set auth.existingSecret=shopping-list \
  --set networking.gateway.hostname=shopping-list.example.com
```

### Networking

The chart supports two networking modes, configured via `networking.type`:

**Gateway API (default)**

```yaml
networking:
  type: gateway
  gateway:
    hostname: shopping-list.example.com
    parentRefs:
      - name: api-gateway
        namespace: nginx-gateway
```

**Ingress**

```yaml
networking:
  type: ingress
  ingress:
    className: nginx
    hostname: shopping-list.example.com
```

## API Reference

All API endpoints require an authenticated session (cookie set at login).

| Method   | Path                    | Description              |
|----------|-------------------------|--------------------------|
| `GET`    | `/api/items`            | List all items           |
| `POST`   | `/api/items`            | Create item `{name, quantity[, after_id]}` |
| `PUT`    | `/api/items/{id}`       | Update item `{name, quantity, checked}` |
| `DELETE` | `/api/items/{id}`       | Delete item              |
| `PATCH`  | `/api/items/reorder`    | Reorder items `{ids: []}` |

## CI/CD

| Workflow                | Trigger                           | Description                                      |
|-------------------------|-----------------------------------|--------------------------------------------------|
| `test.yaml`             | Push / PR                         | golangci-lint, hadolint, helm-lint, go test      |
| `auto-release.yaml`     | Push to `main`                    | Conventional commits → semantic version tag      |
| `release.yaml`          | Tag `v*` / manual                 | Helm package + GitHub Release + version-refs PR  |
| `commitlint.yaml`       | PR                                | Enforce Conventional Commits message format      |
| `renovate.yaml`         | Schedule                          | Automated dependency updates (Go, Docker, Actions, SortableJS) |
| `renovate-go-tidy.yml`  | Renovate PR touching `go.mod`     | Run `go mod tidy` automatically                  |
| `update-sortable.yml`   | Push to non-main branch touching `static/sortable.version` | Download matching `Sortable.min.js` and commit to the branch |

## Versioning

This project uses [Conventional Commits](https://www.conventionalcommits.org/) and automated semantic versioning.

Pushing to `main` automatically determines the next version and creates a tag:

| Commit prefix | Version bump |
|---------------|--------------|
| `feat:` | minor (`0.x.0`) |
| `fix:`, `perf:` | patch (`0.0.x`) |
| `feat!:` / `BREAKING CHANGE` | major (`x.0.0`) |
| `chore:`, `docs:`, `ci:`, etc. | no release |

Once a tag is pushed, the release workflow builds and publishes the Docker image and Helm chart to GHCR, creates a GitHub Release, and opens a PR to update version references in `Chart.yaml` and `README.md`.

## License

[Apache 2.0](LICENSE)
