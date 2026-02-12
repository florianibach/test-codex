# Impulse Pause – MVP (Go)

**Impulse Pause** is a lightweight web app that helps reduce impulse purchases.
You park a purchase idea on a waitlist, set a waiting period, and decide later with a clearer head whether to buy.

This repository includes a runnable MVP baseline with:

- Go web server (Dashboard, Item creation, Insights, Settings, Health)
- Docker + Docker Compose for local startup
- Go unit tests
- Playwright E2E including an **exploratory smoke suite**
- GitHub Actions CI

## What is the app for?

The app helps you slow down spontaneous buying decisions:

1. Capture an item (title, optional price/link/tags/note)
2. Set a waiting period (e.g., 24h, 7 days, 30 days, or custom)
3. After the wait, decide intentionally: **Bought** or **Skipped**
4. Use Insights to see how many purchases you skipped and how much money you saved

You can also store your net hourly wage in settings.
Then the app shows a "Work hours" perspective for priced items.

## Technology decision

For this MVP, **Go** was chosen (C# would also have been possible), because a lean setup with fast build and test cycles was preferred.

## Prerequisites

- Go 1.22+
- Node.js 20+
- npm
- Docker + Docker Compose

## Quick Start

If you just want to run it quickly:

```bash
go run ./cmd/server
```

Then open in your browser: http://127.0.0.1:8080

## Local startup (detailed)

### Run directly with Go

```bash
go run ./cmd/server
```

Optional with a custom DB file:

```bash
DB_PATH=./data/app.db go run ./cmd/server
```

App: http://127.0.0.1:8080

### Run with Docker Compose

```bash
docker compose up --build
```

App: http://127.0.0.1:8080

SQLite DB (persisted via Docker volume): `app-data` at `/app/data/app.db`.

## App flow at a glance

- **Dashboard (`/`)**: All captured items with status, price, "Buy after" timestamp plus search, status/tag filters and sorting
- **Add item (`/items/new`)**: Capture a new purchase idea and set a waiting period
- **Insights (`/insights`)**: Overview of skips, saved amount, and top categories
- **Settings (`/settings/profile`)**: Net hourly wage and optional ntfy notification settings

## Running tests

### Go unit tests

```bash
go test ./...
```

### Optional: Docker Compose integration check (MVP-008 AC1/AC2)

Requires a local Docker installation:

```bash
RUN_DOCKER_TESTS=1 go test ./cmd/server -run TestDockerComposeAppReachableAndPersistsDataAcrossRestart -v
```

### Playwright E2E (exploratory smoke suite)

Install:

```bash
npm ci
npm run setup:e2e:deps
```

If your environment reports missing browser runtime libraries (for example `libatk-1.0.so.0`) or Chromium crashes in headless mode, run the dependency step again after `apt` metadata refresh:

```bash
sudo apt-get update
npx playwright install --with-deps chromium
```

On Ubuntu 24.04 the package name is `libatk1.0-0t64` (not `libatk1.0-0`), and Playwright installs the correct `t64` variants automatically when using `--with-deps`.

Run smoke suite:

```bash
npm run test:e2e:smoke
```

Run monkeyish robustness suite:

```bash
npm run test:e2e:monkeyish
```

The smoke suite checks:

- Navigation across pages
- Browser console errors
- HTTP responses with 4xx/5xx status codes

## CI (GitHub Actions)

Workflow: `.github/workflows/ci.yml`

Pipeline steps:

1. Go setup
2. `go test ./...`
3. Node setup + `npm ci`
4. Playwright browser install (Chromium)
5. `npm run test:e2e:smoke` (with 1 retry in CI so traces are generated for flakes)
6. Upload `playwright-report/` and `test-results/` as CI artifacts
