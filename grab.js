const { chromium } = require('playwright');
const cookieVal = process.env.SESSION_COOKIE || '';
(async() => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  await ctx.addCookies([
    { name: 'session', value: cookieVal, domain: '127.0.0.1', path: '/', httpOnly: true, sameSite: 'Lax' },
    { name: 'session', value: cookieVal, domain: 'localhost', path: '/', httpOnly: true, sameSite: 'Lax' },
    { name: 'session', value: cookieVal, domain: 'timelapse-web', path: '/', httpOnly: true, sameSite: 'Lax' },
  ]);
  const host = process.env.APP_HOST || 'http://timelapse-web:8080';
  const shots = [
    [`${host}/`, 'dashboard.png'],
    [`${host}/live`, 'live.png'],
  ];

  for (const [url, file] of shots) {
    const p = await ctx.newPage({ viewport: { width: 1400, height: 900 } });
    try {
      await p.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 });
      await p.waitForTimeout(1500);
      await p.screenshot({ path: '/screens/' + file, fullPage: true });
      console.log('Captured', file);
    } catch (e) {
      console.error('Failed', file, e);
    } finally {
      await p.close();
    }
  }
  await browser.close();
})();
