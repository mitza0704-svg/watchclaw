// Capture dashboard screenshots in CI (headless Chromium) so UI is verified on
// every push in the cloud — no Dell, no manual screenshots.
import { chromium } from 'playwright';

const BASE = process.env.BASE || 'http://127.0.0.1:8787';
const b = await chromium.launch();
const p = await b.newPage({ viewport: { width: 1440, height: 900 } });

async function shot(name) { await p.screenshot({ path: `shots/${name}.png`, fullPage: false }); }

await p.goto(BASE + '/app', { waitUntil: 'networkidle' });
await p.waitForTimeout(1800);                 // let cytoscape layout settle
await shot('topology-physical');

await p.evaluate(() => window.setLayer && setLayer('connections'));
await p.waitForTimeout(900);
await shot('topology-connections');

await p.evaluate(() => window.openDetail && openDetail('DESKTOP-CI'));
await p.waitForTimeout(1200);
await p.screenshot({ path: 'shots/endpoint-detail.png', fullPage: true });

await p.goto(BASE + '/app', { waitUntil: 'networkidle' });
await p.evaluate(() => window.show && show('sc'));
await p.waitForTimeout(700);
await shot('scripts');

await p.evaluate(() => window.show && show('jb'));
await p.waitForTimeout(700);
await shot('jobs');

await p.goto(BASE + '/', { waitUntil: 'networkidle' });
await p.waitForTimeout(1200);
await p.screenshot({ path: 'shots/landing.png', fullPage: true });

const errs = [];
p.on('console', m => { if (m.type() === 'error') errs.push(m.text()); });
await b.close();
console.log('screenshots captured');
