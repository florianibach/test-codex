# Impulse Pause

[![CI](https://github.com/florianibach/impulse-pause/actions/workflows/ci.yml/badge.svg)](https://github.com/florianibach/impulse-pause/actions/workflows/ci.yml)
[![Docker Image](https://github.com/florianibach/impulse-pause/actions/workflows/docker-image.yml/badge.svg)](https://github.com/florianibach/impulse-pause/actions/workflows/docker-image.yml)
[![Docker Hub Description](https://github.com/florianibach/impulse-pause/actions/workflows/dockerhub-description.yml/badge.svg)](https://github.com/florianibach/impulse-pause/actions/workflows/dockerhub-description.yml)

[GitHub Repo](https://github.com/florianibach/impulse-pause)  
[DockerHub Repo](https://hub.docker.com/r/florianibach/impulse-pause)

pullpulse is a lightweight, self-hosted watcher for Docker Hub repository pull counts.

This project is built and maintained in my free time.
If it helps you or saves you some time, you can support my work on [BuyMeACoffee](https://buymeacoffee.com/florianibach)

Thank you for your support!

## Overview

Impulse Pause is a lightweight web app that helps reduce impulse purchases.
You park a purchase idea on a waiting list, set a waiting period, and decide later with a clearer head whether to buy.

## Features

- Capture items (title, optional price, link, tags, note)
- Set waiting periods (24h, 7 days, 30 days, or custom)
- Decide after the waiting period: Bought or Skipped
- Insights for skipped purchases and saved money
- Optional work-hours perspective based on net hourly wage
- Optional ntfy notifications in profile settings

## Tech Stack

- Backend: Go (net/http)
- UI: Server-rendered HTML templates + CSS
- Storage: SQLite
- Tests: Go tests + Playwright E2E
- Runtime: Docker + Docker Compose

## Getting Started

### Prerequisites

- Go 1.22+
- Node.js 20+
- npm
- Docker + Docker Compose (optional)

### Run locally with Go

```bash
go run ./cmd/server
```

Open: <http://127.0.0.1:8080>

Optional custom DB path:

```bash
DB_PATH=./data/app.db go run ./cmd/server
```

### Docker Compose quickstart example

```yaml
# docker-compose.quickstart.yml
services:
  app:
    build: .
    container_name: impulse-pause
    ports:
      - "8080:8080"
    environment:
      DB_PATH: /app/data/app.db
      ADDR: :8080
    volumes:
      - impulse-pause-data:/app/data
    restart: unless-stopped

volumes:
  impulse-pause-data:
```

Start:

```bash
docker compose -f docker-compose.quickstart.yml up --build -d
```

Logs:

```bash
docker compose -f docker-compose.quickstart.yml logs -f
```

Stop:

```bash
docker compose -f docker-compose.quickstart.yml down
```

You can also use the repository default compose file:

```bash
docker compose up --build
```

## Application Routes

- `/` Dashboard with search, filter, and sorting
- `/items/new` Create a new purchase idea
- `/insights` Overview for skipped items, savings, and top categories
- `/settings/profile` Hourly wage and notification settings
- `/health` Health endpoint

## Testing

### Go tests

```bash
go test ./...
```

### Docker Compose integration test (optional)

```bash
RUN_DOCKER_TESTS=1 go test ./cmd/server -run TestDockerComposeAppReachableAndPersistsDataAcrossRestart -v
```

### Playwright E2E

Install dependencies:

```bash
npm ci
npm run setup:e2e:deps
```

Run smoke suite:

```bash
npm run test:e2e:smoke
```

Run monkeyish suite:

```bash
npm run test:e2e:monkeyish
```

Run all E2E tests:

```bash
npm run test:e2e
```

## CI/CD

- CI workflow: `.github/workflows/ci.yml`
- Docker image workflow: `.github/workflows/docker-image.yml`
- Docker Hub description workflow: `.github/workflows/dockerhub-description.yml`

## Build and run Docker image locally

```bash
docker build -t impulse-pause:local .
```

```bash
docker run --rm -p 8080:8080 impulse-pause:local
```
