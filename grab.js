const { chromium } = require('playwright');

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

const cookieVal = process.env.SESSION_COOKIE || '';
const host = process.env.APP_HOST || 'http://timelapse-web:8080';
const hostname = new URL(host).hostname;

(async() => {
  assert(cookieVal, 'SESSION_COOKIE is required');
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  await ctx.addCookies([{ name: 'session', value: cookieVal, domain: hostname, path: '/', httpOnly: true, sameSite: 'Lax' }]);

  try {
    const dashboard = await ctx.newPage();
    await dashboard.goto(`${host}/`, { waitUntil: 'domcontentloaded', timeout: 30000 });
    assert(!dashboard.url().includes('/login'), 'dashboard redirected to login');
    await dashboard.waitForSelector('.camera-card', { timeout: 10000 });
    const cardIds = await dashboard.locator('.camera-card').evaluateAll(cards => cards.map(card => card.dataset.cam));
    assert(cardIds.length > 0, 'dashboard contains no camera cards');

    const registry = await ctx.request.get(`${host}/api/registry`, { timeout: 10000 });
    assert(registry.ok(), `registry returned HTTP ${registry.status()}`);
    const rows = await registry.json();
    assert(rows.length === cardIds.length, `registry count ${rows.length} != card count ${cardIds.length}`);

    let jpegCount = 0;
    for (const id of cardIds) {
      const thumb = await ctx.request.get(`${host}/camera/${id}/thumb.jpg?ts=${Date.now()}`, { timeout: 10000 });
      assert(thumb.ok(), `thumbnail ${id} returned HTTP ${thumb.status()}`);
      if ((thumb.headers()['content-type'] || '').startsWith('image/jpeg')) jpegCount += 1;
    }
    assert(jpegCount > 0, 'no proxied JPEG thumbnails were returned');
    const diagnose = await ctx.request.get(`${host}/api/camera/${cardIds[0]}/diagnose`, { timeout: 10000 });
    assert(diagnose.ok(), `diagnose returned HTTP ${diagnose.status()}`);
    const diagnoseBody = await diagnose.json();
    assert((diagnoseBody.summary || '').includes('Latest edge frame age'), 'diagnose did not use split-aware edge frame health');
    await dashboard.waitForTimeout(1200);
    await dashboard.screenshot({ path: '/screens/dashboard.png', fullPage: true });
    console.log(`dashboard_ok cards=${cardIds.length} jpeg_thumbs=${jpegCount}`);
    await dashboard.close();

    const live = await ctx.newPage();
    await live.goto(`${host}/live`, { waitUntil: 'domcontentloaded', timeout: 30000 });
    assert(!live.url().includes('/login'), 'live view redirected to login');
    await live.waitForSelector('.tile', { timeout: 10000 });
    const tileCount = await live.locator('.tile').count();
    assert(tileCount === cardIds.length, `live tile count ${tileCount} != card count ${cardIds.length}`);
    await live.waitForTimeout(1800);
    await live.screenshot({ path: '/screens/live.png', fullPage: true });
    console.log(`live_ok tiles=${tileCount}`);
    await live.close();
  } finally {
    await browser.close();
  }
})().catch(err => {
  console.error(err.stack || err);
  process.exit(1);
});
