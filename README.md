# 🛒 Shopping List

A lightweight, self-hosted shopping list web application built with Go and SQLite.

## Features

- **Single-user authentication** — bcrypt password hashing, HMAC-signed session cookies
- **Shopping list management** — add, edit, delete and check off items with quantities
- **Drag & drop reordering** — reorder items via SortableJS
- **Filter view** — hide already-checked items with one click
- **Dark mode** — automatic via `prefers-color-scheme`, toggle persisted in `localStorage`
- **Zero dependencies at runtime** — single static binary with embedded templates and assets
- **SQLite persistence** — no external database required

## Tech Stack

| Layer      | Technology                                |
|------------|-------------------------------------------|
| Backend    | Go 1.22, `net/http` stdlib                |
| Database   | SQLite via `modernc.org/sqlite` (pure Go, CGO-free) |
| Frontend   | Vanilla JS + CSS, [SortableJS](https://sortablejs.github.io/Sortable/) |
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

| Variable           | Default          | Description                                              |
|--------------------|------------------|----------------------------------------------------------|
| `APP_USERNAME`     | `admin`          | Login username                                           |
| `APP_PASSWORD`     | *(required)*     | Login password (plain text; hashed with bcrypt at startup) |
| `APP_SESSION_SECRET` | *(required)*   | HMAC secret for session cookies (min. 32 random bytes)   |
| `DATABASE_PATH`    | `/data/shopping.db` | Path to the SQLite database file                      |
| `APP_ADDR`         | `:8080`          | HTTP listen address                                      |

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

## Kubernetes (Helm)

```bash
helm install shopping-list oci://ghcr.io/steled/charts/shopping-list \
  --version 0.1.0 \
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
  --version 0.1.0 \
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
| `POST`   | `/api/items`            | Create item `{name, quantity}` |
| `PUT`    | `/api/items/{id}`       | Update item `{name, quantity, checked}` |
| `DELETE` | `/api/items/{id}`       | Delete item              |
| `POST`   | `/api/items/reorder`    | Reorder items `{ids: []}` |

## CI/CD

| Workflow           | Trigger                        | Description                                      |
|--------------------|--------------------------------|--------------------------------------------------|
| `test.yaml`        | Push / PR                      | golangci-lint, hadolint, helm-lint, go test      |
| `auto-release.yaml`| Push to `main`                 | Conventional commits → semantic version tag      |
| `release.yaml`     | Tag `v*` / manual              | Helm package + GitHub Release + version-refs PR  |
| `commitlint.yaml`  | PR                             | Enforce Conventional Commits message format      |
| `renovate.yaml`    | Schedule                       | Automated dependency updates                     |
| `renovate-go-tidy.yml` | Renovate PR touching `go.mod` | Run `go mod tidy` automatically              |

## License

[MIT](LICENSE)
