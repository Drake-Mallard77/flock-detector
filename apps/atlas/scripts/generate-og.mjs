// Renders scripts/og-card.html to public/og-card.png at exactly 1200x630.
//
// Run with: node scripts/generate-og.mjs
//
// The PNG is committed rather than generated during the Docker build. Social
// crawlers fetch the image directly, so it has to exist as a static file
// regardless; building it in CI would put a headless browser in the image
// pipeline to produce a file that changes maybe twice a year. Keeping the
// HTML source next to it means the card is still editable and reviewable in
// a diff, which is the part that usually gets lost when someone exports a
// one-off from a design tool.
//
// Playwright is deliberately NOT a dependency of this app. Its postinstall
// downloads browser binaries, which every `npm ci` in the Docker build would
// then pay for — a large, permanent cost for a script that runs about twice
// a year. Install it where you run this:
//
//   npm i --no-save playwright && npx playwright install chromium
//   node scripts/generate-og.mjs

import { chromium } from "playwright";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));
const source = resolve(here, "og-card.html");
const out = resolve(here, "..", "public", "og-card.png");

const browser = await chromium.launch();
const page = await browser.newPage({
  viewport: { width: 1200, height: 630 },
  // Render at 1x. Retina-scaling an OG card just inflates the file for
  // platforms that downscale it again; several also reject images over a
  // few hundred KB.
  deviceScaleFactor: 1,
});

await page.goto(`file://${source}`, { waitUntil: "networkidle" });
await page.screenshot({ path: out });
await browser.close();

console.log(`wrote ${out}`);
