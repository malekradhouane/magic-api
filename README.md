# Magic API

## Prerequisites

- Go 1.24+
- PostgreSQL 12+
- Docker and Docker Compose (for local development)
- Make
- [swag](https://github.com/swaggo/swag) CLI (for Swagger generation)

## Getting Started

### 1. configure

```bash
cp .env.example .env
# Edit .env with your credentials
```

### 2. Start local infrastructure

```bash
make local        # starts PostgreSQL via Docker Compose
```

### 3. Build and run

```bash
make              # builds magic + storeinit binaries
make run          # loads .env and starts the server
```

The API is available at `http://localhost:5002`.

### 4. Database migrations

Migrations run automatically at startup via `storeinit`. To run manually:

```bash
./bin/storeinit --up
```

## API Endpoints

Swagger UI: `http://localhost:5002/swagger/index.html`

## Development

### Run tests

```bash
make test
```

### Regenerate Swagger docs

```bash
swag init --dir ./cmd/magic,. --parseDependency --parseInternal --parseDepth 1 --output docs --outputTypes yaml,go
```

### Adding an OAuth provider

1. Add the provider in `auth/auth.go` → `Init()`
2. No route changes needed — the `/:provider` pattern handles it automatically

### Docker

```bash
# Local dev stack
docker-compose up -d

# Production image
docker build -t magic-api:latest .
docker run --env-file .env -p 5002:5002 magic-api:latest
```

