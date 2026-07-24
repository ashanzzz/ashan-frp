const { chromium } = require('playwright');
(async () => {
  const browser = await chromium.launch({ headless: true, executablePath: 'C:/Users/ashan/AppData/Local/ms-playwright/chromium-1228/chrome-win64/chrome.exe' });
  const page = await browser.newPage({ viewport: { width: 1440, height: 1100 }, deviceScaleFactor: 1 });
  await page.goto('http://127.0.0.1:18081/ui/', { waitUntil: 'networkidle' });
  await page.fill('#login-username', 'admin');
  await page.fill('#login-password', 'admin123456');
  await Promise.all([
    page.waitForResponse(resp => resp.url().includes('/api/v1/auth/login') && resp.status() === 200),
    page.click('#login-form button[type="submit"]')
  ]);
  await page.waitForLoadState('networkidle');
  await page.click('[data-page="dashboard"]');
  await page.waitForSelector('[data-view="dashboard"].active');
  await page.waitForTimeout(600);
  await page.screenshot({ path: 'C:/Users/ashan/Documents/ashan frp/.codex-runtime/screenshots/04-dashboard-stats.png', fullPage: true });
  await browser.close();
})();
