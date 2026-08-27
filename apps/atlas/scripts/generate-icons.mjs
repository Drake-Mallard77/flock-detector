// Renders public/favicon.svg to the raster icons that some platforms still
// require, and writes size proofs for review.
//
// Run with: node scripts/generate-icons.mjs
//
// See generate-og.mjs for why Playwright is not a dependency of this app.
//
// The proofs matter more than they sound. A favicon drawn at a comfortable
// size routinely falls apart at 16px, where it is actually seen: the first
// version of this mark had a catchlight that merged with the outer ring and
// turned the lens into a notched "C". That is invisible until you look at a
// 16px render, so this script produces them and puts them in scratch rather
// than making you squint at a browser tab.

import { chromium } from "playwright";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { readFileSync, mkdirSync } from "node:fs";

const here = dirname(fileURLToPath(import.meta.url));
const publicDir = resolve(here, "..", "public");
const proofDir = resolve(here, "..", ".icon-proofs");

const svg = readFileSync(resolve(publicDir, "favicon.svg"), "utf8");
mkdirSync(proofDir, { recursive: true });

const targets = [
  // iOS ignores SVG favicons entirely and falls back to a screenshot of the
  // page if this is missing.
  { size: 180, out: resolve(publicDir, "apple-touch-icon.png") },
  { size: 16, out: resolve(proofDir, "proof-16.png") },
  { size: 32, out: resolve(proofDir, "proof-32.png") },
  { size: 64, out: resolve(proofDir, "proof-64.png") },
];

const browser = await chromium.launch();

for (const { size, out } of targets) {
  const page = await browser.newPage({
    viewport: { width: size, height: size },
    deviceScaleFactor: 1,
  });
  // Wrapped in a document rather than navigated to directly: a bare SVG file
  // has no <head> or <body>, so there is nothing to size or style.
  const sized = svg.replace(
    "<svg ",
    `<svg width="${size}" height="${size}" style="display:block" `,
  );
  await page.setContent(`<html><body style="margin:0">${sized}</body></html>`);
  await page.screenshot({ path: out });
  await page.close();
}

await browser.close();
console.log(`wrote apple-touch-icon.png and size proofs in ${proofDir}`);
