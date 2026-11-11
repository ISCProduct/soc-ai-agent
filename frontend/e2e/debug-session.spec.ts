import { test, expect } from '@playwright/test';

test('debug session ID initialization', async ({ page }) => {
  // コンソールログを監視
  const logs: string[] = [];
  page.on('console', msg => {
    logs.push(`[${msg.type()}] ${msg.text()}`);
    console.log(`[${msg.type()}] ${msg.text()}`);
  });
  
  // ページエラーを監視
  page.on('pageerror', error => {
    console.error('Page error:', error);
  });
  
  console.log('Navigating to http://localhost:3000...');
  await page.goto('http://localhost:3000');
  
  // ページが読み込まれるまで待機
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(3000);
  
  console.log('\n=== All Console Logs ===');
  logs.forEach(log => console.log(log));
  console.log('========================\n');
  
  // LocalStorageの状態を確認
  const storageState = await page.evaluate(() => {
    const allKeys = Object.keys(localStorage);
    const result: Record<string, unknown> = {};
    allKeys.forEach(key => {
      result[key] = localStorage.getItem(key);
    });
    return result;
  });
  
  console.log('LocalStorage state:', JSON.stringify(storageState, null, 2));
  
  // ページのHTMLを確認
  const bodyText = await page.textContent('body');
  console.log('Page contains "IT業界":', bodyText?.includes('IT業界'));
  
  // スクリーンショット
  await page.screenshot({ path: 'debug-screenshot.png', fullPage: true });
  console.log('Screenshot saved to debug-screenshot.png');
});
