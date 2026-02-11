# MVP Grundgerüst (Go)

Dieses Repository enthält ein lauffähiges MVP-Basisprojekt mit:

- Go-Webserver (Home/About/Health)
- Docker + Docker Compose für lokalen Start
- Unit-Tests (Go)
- Playwright E2E inkl. **exploratory smoke suite**
- GitHub Actions CI

## Technologieentscheidung

Für dieses MVP wurde **Go** gewählt (alternativ wäre C# möglich gewesen), da ein schlankes Setup mit schneller Build-/Test-Zeit gewünscht ist.

## Voraussetzungen

- Go 1.22+
- Node.js 20+
- npm
- Docker + Docker Compose

## Lokal starten

### Direkt mit Go

```bash
go run ./cmd/server
```

App: http://127.0.0.1:8080

### Mit Docker Compose

```bash
docker compose up --build
```

App: http://127.0.0.1:8080

## Tests ausführen

### Go Unit-Tests

```bash
go test ./...
```

### Playwright E2E (exploratory smoke suite)

Installieren:

```bash
npm ci
npx playwright install --with-deps chromium
```

Smoke Suite ausführen:

```bash
npm run test:e2e:smoke
```

Die Smoke Suite prüft:

- Navigation zwischen Seiten
- Console Errors im Browser
- HTTP-Antworten auf 4xx/5xx

## CI (GitHub Actions)

Workflow: `.github/workflows/ci.yml`

Pipeline-Schritte:

1. Go Setup
2. `go test ./...`
3. Node Setup + `npm ci`
4. Playwright Browser-Installation (Chromium)
5. `npm run test:e2e:smoke` (mit 1 Retry in CI, damit bei Flakes ein Trace erzeugt wird)
6. Upload von `playwright-report/` und `test-results/` als CI-Artefakte
