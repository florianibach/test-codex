import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e',
  use: {
    baseURL: process.env.BASE_URL || 'http://127.0.0.1:8080',
    trace: 'on-first-retry'
  },
  webServer: {
    command: 'go run ./cmd/server',
    port: 8080,
    reuseExistingServer: true,
    timeout: 120_000
  }
});
