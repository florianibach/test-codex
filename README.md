# Impulse Pause

[![CI](https://github.com/florianibach/impulse-pause/actions/workflows/ci.yml/badge.svg)](https://github.com/florianibach/impulse-pause/actions/workflows/ci.yml)
[![Docker](https://github.com/florianibach/impulse-pause/actions/workflows/docker.yml/badge.svg)](https://github.com/florianibach/impulse-pause/actions/workflows/docker.yml)

**Impulse Pause** hilft dir dabei, Impulskäufe bewusst zu entschleunigen.
Statt sofort zu kaufen, parkst du Ideen auf einer Warteliste, setzt eine Wartezeit und entscheidest später mit klarem Kopf.

> Bilder, UI-Screenshots und Demo-GIFs kannst du hier später einfach ergänzen.

---

## ✨ Features

- 📥 Kaufideen erfassen (Titel, Preis, Link, Tags, Notiz)
- ⏱️ Wartezeit setzen (z. B. 24h, 7 Tage, 30 Tage oder individuell)
- ✅ Nach Ablauf bewusst entscheiden: **Gekauft** oder **Übersprungen**
- 📊 Insights mit gespartem Betrag und Kategorien
- 💶 Optionale Perspektive: Preis in **Arbeitsstunden** (über Netto-Stundenlohn)
- 🔔 Optional ntfy-Benachrichtigungen (konfigurierbar in den Einstellungen)

---

## 🧰 Tech Stack

- **Backend:** Go (net/http)
- **Templates/UI:** Server-rendered HTML + CSS
- **Datenbank:** SQLite
- **Tests:** Go-Tests + Playwright E2E
- **Container:** Docker + Docker Compose

---

## 🚀 Getting Started

### Voraussetzungen

- Go 1.22+
- Node.js 20+
- npm
- Docker + Docker Compose (optional, aber empfohlen)

### 1) Lokal direkt mit Go starten

```bash
go run ./cmd/server
```

App öffnen: <http://127.0.0.1:8080>

Optional mit eigenem DB-Pfad:

```bash
DB_PATH=./data/app.db go run ./cmd/server
```

### 2) Getting Started mit Docker Compose (Beispiel)

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

Starten:

```bash
docker compose -f docker-compose.quickstart.yml up --build -d
```

Logs ansehen:

```bash
docker compose -f docker-compose.quickstart.yml logs -f
```

Stoppen:

```bash
docker compose -f docker-compose.quickstart.yml down
```

Wenn du das bestehende Repository-Compose verwenden willst, reicht auch:

```bash
docker compose up --build
```

---

## 🗺️ App-Bereiche

- `/` – Dashboard mit Such-, Filter- und Sortieroptionen
- `/items/new` – Neue Kaufidee anlegen
- `/insights` – Übersicht über Übersprünge, Ersparnisse und Top-Kategorien
- `/settings/profile` – Stundenlohn und Benachrichtigungseinstellungen
- `/health` – Health-Endpoint

---

## 🧪 Tests

### Go-Tests

```bash
go test ./...
```

### Docker-Compose Integrationscheck (optional)

```bash
RUN_DOCKER_TESTS=1 go test ./cmd/server -run TestDockerComposeAppReachableAndPersistsDataAcrossRestart -v
```

### Playwright E2E

Installieren:

```bash
npm ci
npm run setup:e2e:deps
```

Smoke-Suite:

```bash
npm run test:e2e:smoke
```

Monkeyish-Suite:

```bash
npm run test:e2e:monkeyish
```

Alle E2E-Tests:

```bash
npm run test:e2e
```

---

## ⚙️ CI/CD

- **CI Workflow** (`.github/workflows/ci.yml`)
  - Go-Tests
  - Node Setup + npm Install
  - Playwright E2E
  - Upload von Test-Artefakten

- **Docker Workflow** (`.github/workflows/docker.yml`)
  - Docker Build bei Push/PR
  - Optionales Pushen in die GitHub Container Registry (nur auf `master`)

---

## 📦 Docker Image lokal bauen

```bash
docker build -t impulse-pause:local .
```

```bash
docker run --rm -p 8080:8080 impulse-pause:local
```

---

## 📄 Lizenz

Interne/Projekt-Lizenz nach Bedarf ergänzen.
