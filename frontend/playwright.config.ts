import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  // 新規作成したテストのみ実行（既存デバッグ用スペックを除外）
  testMatch: ['auth.spec.ts', 'chat.spec.ts', 'resume.spec.ts', 'schedule.spec.ts', 'admin.spec.ts'],
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: 'http://localhost:3000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    actionTimeout: 10000,
    navigationTimeout: 15000,
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  webServer: process.env.CI
    ? {
        command: 'npm run build && node .next/standalone/server.js',
        url: 'http://localhost:3000',
        reuseExistingServer: false,
        timeout: 180000,
        env: {
          PORT: '3000',
          NEXT_PUBLIC_BACKEND_URL: 'http://localhost:3000',
          BACKEND_URL: 'http://localhost:3000',
        },
      }
    : undefined,
});
