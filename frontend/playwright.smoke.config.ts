import { defineConfig, devices } from '@playwright/test'

// 反映後の実環境（staging/production）に対する非破壊スモークテスト用設定。
// playwright.config.ts（localhost向け・webServer起動）とは完全に分離する。
const baseURL = process.env.PLAYWRIGHT_BASE_URL
if (!baseURL) {
  throw new Error('PLAYWRIGHT_BASE_URL is required for smoke tests')
}

export default defineConfig({
  testDir: './e2e/smoke',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  // デプロイ直後はコンテナが再起動したてで Next.js がコールド。ログイン画面の
  // タブは MUI のクライアント描画で SSR の HTML に含まれないため、既定の 5 秒
  // では初回描画に間に合わず落ちることがあった（warm なら 1.3 秒で通る）。
  // actionTimeout / navigationTimeout と同じ水準に引き上げる。
  expect: { timeout: 15000 },
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    actionTimeout: 15000,
    navigationTimeout: 20000,
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
})
